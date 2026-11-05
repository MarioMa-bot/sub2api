package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// fakeSidecar 模拟 Presidio 旁路：按给定词表返回码点偏移
type fakeSidecar struct {
	server *httptest.Server
	calls  atomic.Int32
	delay  time.Duration
	status atomic.Int32
	// entities 词 -> 类型
	entities map[string]string
	lastText atomic.Value
}

func newFakeSidecar(t *testing.T, entities map[string]string) *fakeSidecar {
	t.Helper()
	f := &fakeSidecar{entities: entities}
	f.status.Store(http.StatusOK)
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(int(f.status.Load()))
			return
		}
		f.calls.Add(1)
		var req struct {
			Text     string   `json:"text"`
			Entities []string `json:"entities"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		f.lastText.Store(req.Text)
		if f.delay > 0 {
			select {
			case <-time.After(f.delay):
			case <-r.Context().Done():
				return
			}
		}
		if code := int(f.status.Load()); code != http.StatusOK {
			w.WriteHeader(code)
			return
		}
		out := []SmartEntity{}
		for word, kind := range f.entities {
			for offset := 0; ; {
				i := strings.Index(req.Text[offset:], word)
				if i < 0 {
					break
				}
				byteStart := offset + i
				start := utf8.RuneCountInString(req.Text[:byteStart])
				out = append(out, SmartEntity{Start: start, End: start + utf8.RuneCountInString(word), Kind: kind, Score: 0.85})
				offset = byteStart + len(word)
			}
		}
		_ = json.NewEncoder(w).Encode(out)
	}))
	t.Cleanup(f.server.Close)
	return f
}

func smartPrivacyConfig(url string) *PrivacyMaskingConfig {
	cfg := enabledPrivacyConfig()
	cfg.SmartMaskingEnabled = true
	cfg.SmartSidecarURL = url
	cfg.SmartTimeoutMS = 200
	cfg.SmartMinScore = 0.5
	cfg.SmartEntities = DefaultSmartEntities()
	return cfg
}

func newSmartTestRouter(t *testing.T, store *fakePrivacyMaskingStore, upstream gin.HandlerFunc) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	svc, err := NewPrivacyMaskingService(store)
	require.NoError(t, err)
	mw := NewPrivacyMaskingMiddleware(svc)
	mw.logWriter = func(fn func()) { fn() }
	r := gin.New()
	r.POST("/v1/messages", mw.Handler(func(*gin.Context) PrivacyMaskingIdentity {
		return PrivacyMaskingIdentity{UserID: 7}
	}), upstream)
	return r
}

func TestSmartMasking_PersonMaskedRestoredAndLoggedAsMetadata(t *testing.T) {
	sidecar := newFakeSidecar(t, map[string]string{"John Smith": "PERSON", "Claude": "PERSON"})
	store := &fakePrivacyMaskingStore{cfg: smartPrivacyConfig(sidecar.server.URL)}
	var upstreamBody string
	r := newSmartTestRouter(t, store, func(c *gin.Context) {
		raw, _ := io.ReadAll(c.Request.Body)
		upstreamBody = string(raw)
		c.String(http.StatusOK, "hi "+placeholderPattern.FindString(upstreamBody))
	})

	// model 里的 "Claude" 不在正文字段中，绝不能被改写
	body := `{"model":"Claude","messages":[{"role":"user","content":"Ask John Smith and ` + testSecret + `"}]}`
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body)))

	require.Equal(t, http.StatusOK, w.Code)
	require.NotContains(t, upstreamBody, "John Smith")
	require.NotContains(t, upstreamBody, testSecret)
	require.Contains(t, upstreamBody, `"model":"Claude"`)
	require.Contains(t, upstreamBody, "{{PERSON_")
	require.Contains(t, upstreamBody, "{{API_KEY_")
	require.True(t, json.Valid([]byte(upstreamBody)))
	require.Equal(t, "hi John Smith", w.Body.String())

	require.Len(t, store.logs, 1)
	entry := store.logs[0]
	require.Equal(t, 1, entry.Kinds["PERSON"])
	require.Equal(t, 1, entry.Kinds["API_KEY"])
	encoded, err := json.Marshal(entry)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "John")
	require.NotContains(t, string(encoded), "Smith")

	sent, _ := sidecar.lastText.Load().(string)
	require.NotContains(t, sent, "user", "role must not be sent to the sidecar")
}

func TestSmartMasking_DisabledNeverCallsSidecar(t *testing.T) {
	sidecar := newFakeSidecar(t, map[string]string{"John Smith": "PERSON"})
	cfg := smartPrivacyConfig(sidecar.server.URL)
	cfg.SmartMaskingEnabled = false
	store := &fakePrivacyMaskingStore{cfg: cfg}
	var upstreamBody string
	r := newSmartTestRouter(t, store, func(c *gin.Context) {
		raw, _ := io.ReadAll(c.Request.Body)
		upstreamBody = string(raw)
		c.Status(http.StatusOK)
	})

	body := `{"messages":[{"role":"user","content":"Ask John Smith"}]}`
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body)))

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, body, upstreamBody)
	require.Zero(t, sidecar.calls.Load())
	require.Empty(t, store.logs)
}

func TestSmartMasking_SidecarSlowOrDownFallsBackToRules(t *testing.T) {
	slow := newFakeSidecar(t, map[string]string{"John Smith": "PERSON"})
	slow.delay = 2 * time.Second
	failing := newFakeSidecar(t, map[string]string{"John Smith": "PERSON"})
	failing.status.Store(http.StatusInternalServerError)
	down := httptest.NewServer(http.NotFoundHandler())
	downURL := down.URL
	down.Close()

	for name, url := range map[string]string{"slow": slow.server.URL, "error": failing.server.URL, "down": downURL, "unset": ""} {
		t.Run(name, func(t *testing.T) {
			t.Setenv(SmartSidecarURLEnv, "")
			cfg := smartPrivacyConfig(url)
			cfg.SmartTimeoutMS = 80
			store := &fakePrivacyMaskingStore{cfg: cfg}
			var upstreamBody string
			r := newSmartTestRouter(t, store, func(c *gin.Context) {
				raw, _ := io.ReadAll(c.Request.Body)
				upstreamBody = string(raw)
				c.Status(http.StatusOK)
			})

			body := `{"messages":[{"role":"user","content":"Ask John Smith, key ` + testSecret + `"}]}`
			started := time.Now()
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body)))

			require.Equal(t, http.StatusOK, w.Code, "NER failure must never fail the request")
			require.Less(t, time.Since(started), time.Second, "hard timeout must bound the latency")
			require.Contains(t, upstreamBody, "John Smith", "rules-only fallback")
			require.NotContains(t, upstreamBody, testSecret, "rules still apply")
			require.Len(t, store.logs, 1)
			require.Zero(t, store.logs[0].Kinds["PERSON"])
		})
	}
}

func TestSmartMasking_ChineseUTF8AndEscapedBodies(t *testing.T) {
	sidecar := newFakeSidecar(t, map[string]string{"张伟": "PERSON", "北京": "LOCATION"})
	svc, err := NewPrivacyMaskingService(&fakePrivacyMaskingStore{cfg: smartPrivacyConfig(sidecar.server.URL)})
	require.NoError(t, err)

	for name, body := range map[string]string{
		"utf8":    `{"messages":[{"role":"user","content":[{"type":"text","text":"请联系张伟，他在北京。"}]}]}`,
		"escaped": `{"messages":[{"role":"user","content":"\u8bf7\u8054\u7cfb\u5f20\u4f1f\uff0c\u4ed6\u5728\u5317\u4eac\u3002"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			masked, result, err := svc.MaskJSON(context.Background(), []byte(body))
			require.NoError(t, err)
			require.True(t, json.Valid(masked))
			require.Contains(t, string(masked), "{{PERSON_")
			require.Contains(t, string(masked), "{{LOCATION_")

			var decoded map[string]any
			require.NoError(t, json.Unmarshal(masked, &decoded))
			require.NotContains(t, string(masked), "张伟")
			require.NotContains(t, strings.ToLower(string(masked)), `\u5f20\u4f1f`)

			restored := UnmaskWith(result.Placeholders, string(masked))
			require.Equal(t, body, restored)
		})
	}
}

func TestSmartMasking_ActionsAndEntityFilter(t *testing.T) {
	sidecar := newFakeSidecar(t, map[string]string{"John Smith": "PERSON", "Acme": "ORGANIZATION", "Paris": "LOCATION"})
	cfg := smartPrivacyConfig(sidecar.server.URL)
	cfg.SmartEntities = []string{"PERSON", "ORG"}
	cfg.KindActions["PERSON"] = PrivacyMaskingActionBlock
	cfg.KindActions["ORG"] = PrivacyMaskingActionWarn
	svc, err := NewPrivacyMaskingService(&fakePrivacyMaskingStore{cfg: cfg})
	require.NoError(t, err)

	body := `{"input":"John Smith from Acme lives in Paris"}`
	_, result, err := svc.MaskJSON(context.Background(), []byte(body))
	require.NoError(t, err)
	require.True(t, result.ShouldBlock)
	require.True(t, result.HasWarnings)
	kinds := extractKinds(result.Detections)
	require.ElementsMatch(t, []string{"PERSON", "ORG"}, kinds, "LOCATION is not enabled")
}

func TestSmartMasking_RuleWinsOnOverlapAndWordBoundary(t *testing.T) {
	sidecar := newFakeSidecar(t, map[string]string{"test@example.com": "PERSON", "Ann": "PERSON"})
	cfg := smartPrivacyConfig(sidecar.server.URL)
	cfg.MaskPersonal = true
	svc, err := NewPrivacyMaskingService(&fakePrivacyMaskingStore{cfg: cfg})
	require.NoError(t, err)

	body := `{"content":"mail test@example.com about the Annual report"}`
	masked, result, err := svc.MaskJSON(context.Background(), []byte(body))
	require.NoError(t, err)
	require.Contains(t, string(masked), "{{EMAIL_")
	require.NotContains(t, string(masked), "{{PERSON_")
	require.Contains(t, string(masked), "Annual")
	require.Len(t, result.Detections, 1)
}

func TestCollectSmartSegments_OnlyTextFieldsWithRawOffsets(t *testing.T) {
	body := []byte(`{"model":"m","system":[{"type":"text","text":"sys \"q\""}],"messages":[{"role":"user","content":"hi"},{"role":"assistant","content":[{"type":"tool_use","name":"x","input":{"query":"q1"}}]}]}`)
	segs := collectSmartSegments(body)
	var texts []string
	for _, seg := range segs {
		texts = append(texts, seg.text)
		raw := string(body[seg.rawStart:seg.rawEnd])
		var decoded string
		require.NoError(t, json.Unmarshal([]byte(`"`+raw+`"`), &decoded))
		require.Equal(t, seg.text, decoded)
	}
	require.Equal(t, []string{`sys "q"`, "hi", "q1"}, texts)
}

func TestCollectSmartSegments_KeepsLatestWithinBudget(t *testing.T) {
	old := strings.Repeat("a", smartMaxScanBytes)
	body, _ := json.Marshal(map[string]any{"messages": []map[string]string{{"content": old}, {"content": "latest"}}})
	segs := collectSmartSegments(body)
	require.Len(t, segs, 1)
	require.Equal(t, "latest", segs[0].text)
}

func TestValidateSmartConfig(t *testing.T) {
	cfg := enabledPrivacyConfig()
	cfg.SmartSidecarURL = "ftp://x"
	require.ErrorIs(t, ValidatePrivacyMaskingConfig(cfg), ErrPrivacyMaskingInvalidConfig)
	cfg.SmartSidecarURL = "http://privacy-ner:5002"
	cfg.SmartTimeoutMS = 1
	require.ErrorIs(t, ValidatePrivacyMaskingConfig(cfg), ErrPrivacyMaskingInvalidConfig)
	cfg.SmartTimeoutMS = 80
	cfg.SmartMinScore = 1.5
	require.ErrorIs(t, ValidatePrivacyMaskingConfig(cfg), ErrPrivacyMaskingInvalidConfig)
	cfg.SmartMinScore = 0.6
	cfg.SmartEntities = []string{"bad kind"}
	require.ErrorIs(t, ValidatePrivacyMaskingConfig(cfg), ErrPrivacyMaskingInvalidConfig)
	cfg.SmartEntities = []string{"person", "organization"}
	require.NoError(t, ValidatePrivacyMaskingConfig(cfg))
}

func TestCheckSmartSidecar(t *testing.T) {
	sidecar := newFakeSidecar(t, nil)
	svc, err := NewPrivacyMaskingService(&fakePrivacyMaskingStore{cfg: smartPrivacyConfig(sidecar.server.URL)})
	require.NoError(t, err)
	target, _, err := svc.CheckSmartSidecar(context.Background(), "")
	require.NoError(t, err)
	require.Equal(t, sidecar.server.URL, target)

	sidecar.status.Store(http.StatusServiceUnavailable)
	_, _, err = svc.CheckSmartSidecar(context.Background(), "")
	require.Error(t, err)
}
