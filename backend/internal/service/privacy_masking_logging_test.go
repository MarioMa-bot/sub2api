package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// fakePrivacyMaskingStore 内存实现，用于验证中间件写日志与配置加载
type fakePrivacyMaskingStore struct {
	mu        sync.Mutex
	cfg       *PrivacyMaskingConfig
	cfgErr    error
	loadCount int
	logs      []*PrivacyMaskingLog
}

func (f *fakePrivacyMaskingStore) GetGlobalConfig(context.Context) (*PrivacyMaskingConfig, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.loadCount++
	if f.cfgErr != nil {
		return nil, f.cfgErr
	}
	return f.cfg, nil
}

func (f *fakePrivacyMaskingStore) SaveGlobalConfig(_ context.Context, cfg *PrivacyMaskingConfig) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cfg = cfg
	return nil
}

func (f *fakePrivacyMaskingStore) InsertLog(_ context.Context, log *PrivacyMaskingLog) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.logs = append(f.logs, log)
	return nil
}

func (f *fakePrivacyMaskingStore) ListLogs(context.Context, PrivacyMaskingLogFilter) ([]PrivacyMaskingLog, int64, error) {
	return nil, 0, nil
}

func (f *fakePrivacyMaskingStore) ListEndpointSummary(context.Context, *time.Time, *time.Time) ([]PrivacyMaskingEndpointSummary, error) {
	return nil, nil
}

func (f *fakePrivacyMaskingStore) GetStats(context.Context, time.Time) (*PrivacyMaskingStats, error) {
	return &PrivacyMaskingStats{}, nil
}

func enabledPrivacyConfig() *PrivacyMaskingConfig {
	return &PrivacyMaskingConfig{
		Enabled:     true,
		MaskSecrets: true,
		MaskedKinds: map[string]bool{},
		KindActions: map[string]PrivacyMaskingAction{},
	}
}

const testSecret = "sk-ant-api03-abcdefghijklmnopqrstuvwxyz0123456789"

func newPrivacyTestRouter(t *testing.T, store *fakePrivacyMaskingStore, upstream gin.HandlerFunc) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	svc, err := NewPrivacyMaskingService(store)
	require.NoError(t, err)
	mw := NewPrivacyMaskingMiddleware(svc)
	mw.logWriter = func(fn func()) { fn() }

	r := gin.New()
	groupID := int64(9)
	resolver := func(c *gin.Context) PrivacyMaskingIdentity {
		keyID := int64(42)
		return PrivacyMaskingIdentity{UserID: 7, APIKeyID: &keyID, GroupID: &groupID}
	}
	r.POST("/v1/messages", mw.Handler(resolver), upstream)
	r.POST("/v1beta/models/*modelAction", mw.Handler(resolver), upstream)
	r.POST("/v1/upload", mw.Handler(resolver), upstream)
	return r
}

func TestPrivacyMaskingMiddleware_MasksForwardsAndLogsMetadataOnly(t *testing.T) {
	store := &fakePrivacyMaskingStore{cfg: enabledPrivacyConfig()}
	var upstreamBody string
	r := newPrivacyTestRouter(t, store, func(c *gin.Context) {
		raw, _ := io.ReadAll(c.Request.Body)
		upstreamBody = string(raw)
		// 模拟上游回显占位符，并带上原始 Content-Length
		echo := placeholderPattern.FindString(upstreamBody)
		c.Header("Content-Length", "999")
		c.String(http.StatusOK, "echo:"+echo)
	})

	body := `{"model":"claude-sonnet-4","messages":[{"role":"user","content":"key ` + testSecret + `"}]}`
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body)))

	require.Equal(t, http.StatusOK, w.Code)
	require.NotContains(t, upstreamBody, testSecret, "upstream must never see the raw secret")
	require.Contains(t, upstreamBody, "{{API_KEY_")
	require.Equal(t, "echo:"+testSecret, w.Body.String(), "response placeholder must be restored")
	require.Empty(t, w.Header().Get("Content-Length"))

	require.Len(t, store.logs, 1)
	entry := store.logs[0]
	require.Equal(t, "/v1/messages", entry.Endpoint)
	require.Equal(t, "claude-sonnet-4", entry.Model)
	require.Equal(t, int64(7), entry.UserID)
	require.Equal(t, int64(42), *entry.APIKeyID)
	require.Equal(t, int64(9), *entry.GroupID)
	require.Equal(t, 1, entry.Kinds["API_KEY"])
	require.Equal(t, 1, entry.MaskedCount)
	require.False(t, entry.RequestBlocked)

	encoded, err := json.Marshal(entry)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), testSecret)
	require.NotContains(t, string(encoded), testSecret[8:], "log must not contain any meaningful part of the secret")
}

func TestPrivacyMaskingMiddleware_BlockRuleRejectsAndLogsBlocked(t *testing.T) {
	cfg := enabledPrivacyConfig()
	cfg.KindActions["API_KEY"] = PrivacyMaskingActionBlock
	store := &fakePrivacyMaskingStore{cfg: cfg}
	called := false
	r := newPrivacyTestRouter(t, store, func(c *gin.Context) { called = true })

	body := `{"contents":[{"parts":[{"text":"` + testSecret + `"}]}]}`
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-2.5-pro:streamGenerateContent", strings.NewReader(body)))

	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "privacy_masking_blocked")
	require.False(t, called, "blocked request must not reach upstream handler")
	require.Len(t, store.logs, 1)
	require.True(t, store.logs[0].RequestBlocked)
	require.Equal(t, "/v1beta/models/{model}:streamGenerateContent", store.logs[0].Endpoint)
	require.Equal(t, "gemini-2.5-pro", store.logs[0].Model)
}

func TestPrivacyMaskingMiddleware_WarnOnlyForwardsOriginalAndLogs(t *testing.T) {
	cfg := enabledPrivacyConfig()
	cfg.KindActions["API_KEY"] = PrivacyMaskingActionWarn
	store := &fakePrivacyMaskingStore{cfg: cfg}
	var upstreamBody string
	r := newPrivacyTestRouter(t, store, func(c *gin.Context) {
		raw, _ := io.ReadAll(c.Request.Body)
		upstreamBody = string(raw)
		c.Status(http.StatusOK)
	})
	body := `{"model":"m","input":"` + testSecret + `"}`
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body)))

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, body, upstreamBody)
	require.Len(t, store.logs, 1)
	require.Equal(t, 1, store.logs[0].WarnedCount)
	require.Equal(t, PrivacyMaskingActionWarn, store.logs[0].Actions["API_KEY"])
}

func TestPrivacyMaskingMiddleware_NoDetectionOrDisabledWritesNoLog(t *testing.T) {
	for name, cfg := range map[string]*PrivacyMaskingConfig{
		"disabled":     {MaskSecrets: true},
		"no detection": enabledPrivacyConfig(),
	} {
		t.Run(name, func(t *testing.T) {
			store := &fakePrivacyMaskingStore{cfg: cfg}
			r := newPrivacyTestRouter(t, store, func(c *gin.Context) { c.Status(http.StatusOK) })
			body := `{"model":"m","input":"hello"}`
			if name == "disabled" {
				body = `{"model":"m","input":"` + testSecret + `"}`
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body)))
			require.Equal(t, http.StatusOK, w.Code)
			require.Empty(t, store.logs)
		})
	}
}

func TestPrivacyMaskingMiddleware_NonJSONBodyPassesThrough(t *testing.T) {
	store := &fakePrivacyMaskingStore{cfg: enabledPrivacyConfig()}
	var upstreamBody string
	r := newPrivacyTestRouter(t, store, func(c *gin.Context) {
		raw, _ := io.ReadAll(c.Request.Body)
		upstreamBody = string(raw)
		c.Status(http.StatusOK)
	})
	body := "--boundary\r\nContent-Disposition: form-data; name=\"file\"\r\n\r\nbinary\r\n--boundary--"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/upload", strings.NewReader(body)))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, body, upstreamBody)
	require.Empty(t, store.logs)
}

func TestPrivacyMaskingMiddleware_ConfigUnavailableFailsClosed(t *testing.T) {
	store := &fakePrivacyMaskingStore{cfgErr: errors.New("db down")}
	called := false
	r := newPrivacyTestRouter(t, store, func(c *gin.Context) { called = true })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"input":"x"}`)))
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	require.False(t, called)
}

func TestPrivacyMaskingMiddleware_StreamingRestoresSplitPlaceholder(t *testing.T) {
	store := &fakePrivacyMaskingStore{cfg: enabledPrivacyConfig()}
	w := httptest.NewRecorder()
	var midStream []string
	r := newPrivacyTestRouter(t, store, func(c *gin.Context) {
		raw, _ := io.ReadAll(c.Request.Body)
		ph := placeholderPattern.FindString(string(raw))
		require.NotEmpty(t, ph)
		c.Header("Content-Type", "text/event-stream")
		c.Status(http.StatusOK)
		// 占位符被拆成两次写入，中间 Flush
		_, _ = c.Writer.WriteString("data: {\"delta\":\"" + ph[:7])
		c.Writer.Flush()
		midStream = append(midStream, w.Body.String())
		_, _ = c.Writer.WriteString(ph[7:] + "\"}\n\n")
		c.Writer.Flush()
		midStream = append(midStream, w.Body.String())
		_, _ = c.Writer.WriteString("data: [DONE]\n\n")
	})

	body := `{"model":"m","stream":true,"input":"` + testSecret + `"}`
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body)))

	// 未闭合的占位符片段被暂存，其余内容立即下发（不缓冲整个响应）
	require.Equal(t, []string{
		"data: {\"delta\":\"",
		"data: {\"delta\":\"" + testSecret + "\"}\n\n",
	}, midStream)
	require.Equal(t, "data: {\"delta\":\""+testSecret+"\"}\n\ndata: [DONE]\n\n", w.Body.String())
}

func TestSplitPartialPlaceholder(t *testing.T) {
	cases := []struct{ in, emit, keep string }{
		{"hello", "hello", ""},
		{"abc{", "abc", "{"},
		{"abc{{API_KE", "abc", "{{API_KE"},
		{"abc{{API_KEY_abcd1234}", "abc", "{{API_KEY_abcd1234}"},
		{"abc{{API_KEY_abcd1234}}", "abc{{API_KEY_abcd1234}}", ""},
		{`x{{not a placeholder"`, `x{{not a placeholder"`, ""},
		{"{{" + strings.Repeat("A", 80), "{{" + strings.Repeat("A", 80), ""},
	}
	for _, tc := range cases {
		emit, keep := splitPartialPlaceholder([]byte(tc.in))
		require.Equal(t, tc.emit, string(emit), tc.in)
		require.Equal(t, tc.keep, string(keep), tc.in)
	}
}

func TestPrivacyMaskingService_LoadConfigFromStore(t *testing.T) {
	store := &fakePrivacyMaskingStore{cfg: enabledPrivacyConfig()}
	svc, err := NewPrivacyMaskingService(store)
	require.NoError(t, err)

	cfg, err := svc.LoadConfig(context.Background())
	require.NoError(t, err)
	require.True(t, cfg.Enabled)
	_, _ = svc.LoadConfig(context.Background())
	require.Equal(t, 1, store.loadCount, "config should be cached")

	// 保存后立即失效并重新加载
	updated := enabledPrivacyConfig()
	updated.Enabled = false
	require.NoError(t, svc.SaveConfig(context.Background(), updated))
	cfg, err = svc.LoadConfig(context.Background())
	require.NoError(t, err)
	require.False(t, cfg.Enabled)
	require.Equal(t, 2, store.loadCount)

	// 数据库故障时沿用上次成功的配置
	store.cfgErr = errors.New("db down")
	svc.InvalidateConfig()
	cfg, err = svc.LoadConfig(context.Background())
	require.NoError(t, err)
	require.False(t, cfg.Enabled)
}

func TestPrivacyMaskingService_SaveConfigValidates(t *testing.T) {
	svc, err := NewPrivacyMaskingService(&fakePrivacyMaskingStore{})
	require.NoError(t, err)
	bad := []*PrivacyMaskingConfig{
		{KindActions: map[string]PrivacyMaskingAction{"API_KEY": "drop"}},
		{CustomRules: []PrivacyMaskingRule{{Pattern: "([", Action: PrivacyMaskingActionMask}}},
		{CustomRules: []PrivacyMaskingRule{{Pattern: "x", Kind: "bad kind"}}},
	}
	for _, cfg := range bad {
		require.ErrorIs(t, svc.SaveConfig(context.Background(), cfg), ErrPrivacyMaskingInvalidConfig)
	}
}

func TestPrivacyMaskingService_OverlappingRulesDoNotCorruptText(t *testing.T) {
	svc, err := NewPrivacyMaskingService(nil)
	require.NoError(t, err)
	svc.applyConfig(enabledPrivacyConfig())

	// sk-ant-... 同时命中 Anthropic、OpenAI 和通用长串规则
	input := `{"text":"a ` + testSecret + ` b"}`
	masked, result, err := svc.MaskJSON(context.Background(), []byte(input))
	require.NoError(t, err)
	require.Len(t, result.Detections, 1)
	require.Regexp(t, `^\{"text":"a \{\{API_KEY_[a-z0-9]{8}\}\} b"\}$`, string(masked))
	require.Equal(t, input, UnmaskWith(result.Placeholders, string(masked)))
}

func TestPrivacyMaskingService_GenericRuleSkipsBase64(t *testing.T) {
	svc, err := NewPrivacyMaskingService(nil)
	require.NoError(t, err)
	svc.applyConfig(enabledPrivacyConfig())

	image := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg/abcdefghijklmnopqrstuvwxyz0123456789+ABCDEFGHIJKLMNOPQRSTUVWXYZ012345=="
	result, err := svc.Mask(context.Background(), `{"data":"`+image+`"}`)
	require.NoError(t, err)
	require.Empty(t, result.Detections)
}

func TestPrivacyMaskingService_MaskedKindsCanDisableKind(t *testing.T) {
	svc, err := NewPrivacyMaskingService(nil)
	require.NoError(t, err)
	cfg := enabledPrivacyConfig()
	cfg.MaskedKinds["API_KEY"] = false
	svc.applyConfig(cfg)
	result, err := svc.Mask(context.Background(), testSecret)
	require.NoError(t, err)
	require.Empty(t, result.Detections)
}

func TestPrivacyMaskingService_MaskJSONFailsClosed(t *testing.T) {
	svc, err := NewPrivacyMaskingService(nil)
	require.NoError(t, err)
	cfg := enabledPrivacyConfig()
	// 自定义规则跨越 JSON 结构，替换后不再是合法 JSON
	cfg.CustomRules = []PrivacyMaskingRule{{Pattern: `"x":"secret`, Kind: KindCustom, Action: PrivacyMaskingActionMask}}
	svc.applyConfig(cfg)
	out, _, err := svc.MaskJSON(context.Background(), []byte(`{"x":"secret"}`))
	require.Error(t, err)
	require.Nil(t, out, "must never fall back to the unmasked body")
}

func TestCreateSamplePrefix_OnlyPublicPrefixes(t *testing.T) {
	require.Equal(t, "sk-a...", CreateSamplePrefix(KindAPIKey, testSecret, 4))
	require.Empty(t, CreateSamplePrefix(KindPassword, "hunter2hunter2hunter2", 4))
	require.Empty(t, CreateSamplePrefix(KindEmail, "someone@example.com", 4))
	require.Empty(t, CreateSamplePrefix(KindAPIKey, "short", 4))
}
