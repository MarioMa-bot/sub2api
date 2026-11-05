package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

// 智能脱敏（L2）：规则脱敏（L1）之外，可选调用 Presidio 旁路识别人名、地名、机构等实体。
// 旁路超时、出错或未配置时静默降级为仅规则脱敏，绝不因 NER 失败而拒绝请求。

const (
	DefaultSmartTimeoutMS = 80
	DefaultSmartMinScore  = 0.6
	SmartTimeoutMinMS     = 10
	SmartTimeoutMaxMS     = 5000

	// SmartSidecarURLEnv 管理后台未填写旁路地址时使用的环境变量
	SmartSidecarURLEnv = "PRIVACY_MASKING_SMART_URL"

	// smartMaxScanBytes 单次请求送检的文本上限；超出时优先保留最新的消息
	smartMaxScanBytes = 32 * 1024
	// smartWarnInterval 降级告警日志的最小间隔，避免旁路故障时刷屏
	smartWarnInterval = 30 * time.Second
)

const (
	KindPerson   PrivacyMaskingKind = "PERSON"
	KindLocation PrivacyMaskingKind = "LOCATION"
	KindOrg      PrivacyMaskingKind = "ORG"
)

// DefaultSmartEntities 默认启用的智能实体类型
func DefaultSmartEntities() []string {
	return []string{string(KindPerson), string(KindLocation), string(KindOrg)}
}

// smartTextKeys 只扫描这些字段下的字符串（消息正文、系统提示等），model、role、tool 名称等不送检、不改写
var smartTextKeys = map[string]bool{
	"content":      true,
	"text":         true,
	"system":       true,
	"prompt":       true,
	"input":        true,
	"instructions": true,
	"query":        true,
}

// SmartDetectRequest 旁路识别请求
type SmartDetectRequest struct {
	URL      string
	Text     string
	Entities []string
	MinScore float64
}

// SmartEntity 旁路返回的实体；Start/End 为 Unicode 码点偏移（左闭右开）
type SmartEntity struct {
	Start int     `json:"start"`
	End   int     `json:"end"`
	Kind  string  `json:"kind"`
	Score float64 `json:"score"`
}

// SmartDetector 智能实体识别器（L2）
type SmartDetector interface {
	Detect(ctx context.Context, req SmartDetectRequest) ([]SmartEntity, error)
	Health(ctx context.Context, baseURL string) error
}

// presidioSmartDetector 通过 HTTP 调用 deploy/presidio 旁路
type presidioSmartDetector struct {
	client *http.Client
}

// NewPresidioSmartDetector 创建旁路客户端。旁路在内网，不走 HTTP(S)_PROXY。
func NewPresidioSmartDetector() SmartDetector {
	transport := &http.Transport{
		Proxy:               nil,
		DialContext:         (&net.Dialer{Timeout: 2 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		MaxIdleConns:        64,
		MaxIdleConnsPerHost: 64,
		IdleConnTimeout:     90 * time.Second,
	}
	return &presidioSmartDetector{client: &http.Client{Transport: transport}}
}

func (d *presidioSmartDetector) Detect(ctx context.Context, req SmartDetectRequest) ([]SmartEntity, error) {
	payload, err := json.Marshal(map[string]any{
		"text":      req.Text,
		"entities":  req.Entities,
		"min_score": req.MinScore,
	})
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(req.URL, "/")+"/analyze", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := d.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("sidecar returned HTTP %d", resp.StatusCode)
	}
	var entities []SmartEntity
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&entities); err != nil {
		return nil, fmt.Errorf("decode sidecar response: %w", err)
	}
	return entities, nil
}

func (d *presidioSmartDetector) Health(ctx context.Context, baseURL string) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := d.client.Do(httpReq)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("sidecar returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// ResolveSmartSidecarURL 优先使用后台配置，其次环境变量
func ResolveSmartSidecarURL(cfg *PrivacyMaskingConfig) string {
	if cfg != nil {
		if u := strings.TrimSpace(cfg.SmartSidecarURL); u != "" {
			return u
		}
	}
	return strings.TrimSpace(os.Getenv(SmartSidecarURLEnv))
}

func smartTimeout(cfg *PrivacyMaskingConfig) time.Duration {
	ms := cfg.SmartTimeoutMS
	if ms <= 0 {
		ms = DefaultSmartTimeoutMS
	}
	if ms < SmartTimeoutMinMS {
		ms = SmartTimeoutMinMS
	}
	if ms > SmartTimeoutMaxMS {
		ms = SmartTimeoutMaxMS
	}
	return time.Duration(ms) * time.Millisecond
}

func normalizeSmartKind(kind string) string {
	kind = strings.ToUpper(strings.TrimSpace(kind))
	if kind == "ORGANIZATION" {
		return string(KindOrg)
	}
	return kind
}

func validateSmartConfig(cfg *PrivacyMaskingConfig) error {
	if u := strings.TrimSpace(cfg.SmartSidecarURL); u != "" {
		parsed, err := url.Parse(u)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return fmt.Errorf("%w: smart sidecar url must be an http(s) URL", ErrPrivacyMaskingInvalidConfig)
		}
	}
	if cfg.SmartTimeoutMS != 0 && (cfg.SmartTimeoutMS < SmartTimeoutMinMS || cfg.SmartTimeoutMS > SmartTimeoutMaxMS) {
		return fmt.Errorf("%w: smart timeout must be between %d and %d ms", ErrPrivacyMaskingInvalidConfig, SmartTimeoutMinMS, SmartTimeoutMaxMS)
	}
	if cfg.SmartMinScore < 0 || cfg.SmartMinScore > 1 {
		return fmt.Errorf("%w: smart min score must be between 0 and 1", ErrPrivacyMaskingInvalidConfig)
	}
	for _, kind := range cfg.SmartEntities {
		if !privacyMaskingKindPattern.MatchString(normalizeSmartKind(kind)) {
			return fmt.Errorf("%w: invalid smart entity %q", ErrPrivacyMaskingInvalidConfig, kind)
		}
	}
	return nil
}

// smartSegment 请求体中一个待扫描的 JSON 字符串
type smartSegment struct {
	rawStart, rawEnd int // 不含引号的原始字节区间
	text             string
}

// collectSmartSegments 收集正文类字段的字符串值；超出上限时保留靠后的（最新的）消息
func collectSmartSegments(body []byte) []smartSegment {
	var segments []smartSegment
	var walk func(v gjson.Result, key string)
	walk = func(v gjson.Result, key string) {
		switch {
		case v.Type == gjson.String:
			if smartTextKeys[key] && v.Index > 0 && len(v.Raw) >= 2 && strings.TrimSpace(v.Str) != "" {
				segments = append(segments, smartSegment{rawStart: v.Index + 1, rawEnd: v.Index + len(v.Raw) - 1, text: v.Str})
			}
		case v.IsArray():
			v.ForEach(func(_, item gjson.Result) bool {
				walk(item, key)
				return true
			})
		case v.IsObject():
			v.ForEach(func(k, item gjson.Result) bool {
				walk(item, k.String())
				return true
			})
		}
	}
	walk(gjson.ParseBytes(body), "")

	total := 0
	for i := len(segments) - 1; i >= 0; i-- {
		total += len(segments[i].text) + 1
		if total > smartMaxScanBytes {
			return segments[i+1:]
		}
	}
	return segments
}

// detectSmart 调用旁路并把实体映射回原始请求体中的字节区间；任何失败都返回 nil（降级为仅规则）
func (s *PrivacyMaskingService) detectSmart(ctx context.Context, cfg *PrivacyMaskingConfig, body []byte) []MaskingDetection {
	if s.smart == nil || !cfg.SmartMaskingEnabled || len(cfg.SmartEntities) == 0 {
		return nil
	}
	sidecarURL := ResolveSmartSidecarURL(cfg)
	if sidecarURL == "" {
		s.warnSmartDegraded(errors.New("sidecar url not configured"))
		return nil
	}
	segments := collectSmartSegments(body)
	if len(segments) == 0 {
		return nil
	}

	var joined strings.Builder
	runeStarts := make([]int, len(segments))
	runeCursor := 0
	for i, seg := range segments {
		if i > 0 {
			_ = joined.WriteByte('\n')
			runeCursor++
		}
		runeStarts[i] = runeCursor
		_, _ = joined.WriteString(seg.text)
		runeCursor += utf8.RuneCountInString(seg.text)
	}

	enabled := make(map[string]bool, len(cfg.SmartEntities))
	for _, kind := range cfg.SmartEntities {
		enabled[normalizeSmartKind(kind)] = true
	}
	entities := make([]string, 0, len(enabled))
	for kind := range enabled {
		entities = append(entities, kind)
	}
	sort.Strings(entities)

	callCtx, cancel := context.WithTimeout(ctx, smartTimeout(cfg))
	defer cancel()
	found, err := s.smart.Detect(callCtx, SmartDetectRequest{
		URL: sidecarURL, Text: joined.String(), Entities: entities, MinScore: cfg.SmartMinScore,
	})
	if err != nil {
		s.warnSmartDegraded(err)
		return nil
	}

	segRunes := make(map[int][]rune)
	seen := make(map[string]bool)
	var detections []MaskingDetection
	for _, ent := range found {
		kind := normalizeSmartKind(ent.Kind)
		if !enabled[kind] || !privacyMaskingKindPattern.MatchString(kind) || ent.Score < cfg.SmartMinScore || ent.Start >= ent.End {
			continue
		}
		idx := sort.Search(len(segments), func(i int) bool { return runeStarts[i] > ent.Start }) - 1
		if idx < 0 {
			continue
		}
		runes, ok := segRunes[idx]
		if !ok {
			runes = []rune(segments[idx].text)
			segRunes[idx] = runes
		}
		start, end := ent.Start-runeStarts[idx], ent.End-runeStarts[idx]
		if start < 0 || end > len(runes) {
			continue
		}
		value := strings.TrimSpace(string(runes[start:end]))
		if utf8.RuneCountInString(value) < 2 || isAllowedValue(cfg, value) {
			continue
		}
		dedupeKey := fmt.Sprintf("%d|%s|%s", idx, kind, value)
		if seen[dedupeKey] {
			continue
		}
		seen[dedupeKey] = true
		detections = append(detections, findSmartOccurrences(body, segments[idx], PrivacyMaskingKind(kind), value, smartActionFor(cfg, PrivacyMaskingKind(kind)))...)
	}
	return detections
}

func smartActionFor(cfg *PrivacyMaskingConfig, kind PrivacyMaskingKind) PrivacyMaskingAction {
	if action, ok := cfg.KindActions[string(kind)]; ok && action.IsValid() {
		return action
	}
	return PrivacyMaskingActionMask
}

// findSmartOccurrences 在该字符串的原始 JSON 字节中查找实体（兼容 UTF-8 直写与 \uXXXX 转义两种编码）
func findSmartOccurrences(body []byte, seg smartSegment, kind PrivacyMaskingKind, value string, action PrivacyMaskingAction) []MaskingDetection {
	raw := string(body[seg.rawStart:seg.rawEnd])
	var out []MaskingDetection
	for _, encoded := range smartEncodings(value) {
		for offset := 0; offset < len(raw); {
			i := strings.Index(raw[offset:], encoded)
			if i < 0 {
				break
			}
			start := offset + i
			end := start + len(encoded)
			offset = end
			if !smartWordBoundary(raw, start, end) {
				continue
			}
			out = append(out, MaskingDetection{
				Kind: kind, Value: encoded, Start: seg.rawStart + start, End: seg.rawStart + end, Action: action,
			})
		}
	}
	return out
}

func smartEncodings(value string) []string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(value)
	plain := strings.TrimSuffix(strings.TrimSpace(buf.String()), "\"")
	plain = strings.TrimPrefix(plain, "\"")

	encodings := []string{plain}
	for _, hexFormat := range []string{`\u%04x`, `\u%04X`} {
		escaped := asciiEscapeJSON(plain, hexFormat)
		if !containsString(encodings, escaped) {
			encodings = append(encodings, escaped)
		}
	}
	return encodings
}

// asciiEscapeJSON 生成 ensure_ascii 风格的编码（如 Python json.dumps 默认输出的 \u5f20）
func asciiEscapeJSON(s, hexFormat string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r < utf8.RuneSelf:
			_ = b.WriteByte(byte(r))
		case r > 0xFFFF:
			r1, r2 := utf16.EncodeRune(r)
			_, _ = fmt.Fprintf(&b, hexFormat+hexFormat, r1, r2)
		default:
			_, _ = fmt.Fprintf(&b, hexFormat, r)
		}
	}
	return b.String()
}

func containsString(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

func isASCIIWordByte(ch byte) bool {
	return ch == '_' || (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

// smartWordBoundary 英文实体两端不能紧贴字母数字（避免 "Ann" 命中 "Annual"）；中文不做限制
func smartWordBoundary(raw string, start, end int) bool {
	if isASCIIWordByte(raw[start]) && start > 0 && isASCIIWordByte(raw[start-1]) {
		return false
	}
	if isASCIIWordByte(raw[end-1]) && end < len(raw) && isASCIIWordByte(raw[end]) {
		return false
	}
	return true
}

var smartLastWarn atomic.Int64

func (s *PrivacyMaskingService) warnSmartDegraded(err error) {
	now := time.Now().UnixNano()
	last := smartLastWarn.Load()
	if now-last < int64(smartWarnInterval) || !smartLastWarn.CompareAndSwap(last, now) {
		return
	}
	logger.L().Warn("privacy masking: smart detection unavailable, falling back to rules only", zap.Error(err))
}

// CheckSmartSidecar 管理后台检测旁路连通性；url 为空时使用当前配置/环境变量
func (s *PrivacyMaskingService) CheckSmartSidecar(ctx context.Context, rawURL string) (string, time.Duration, error) {
	target := strings.TrimSpace(rawURL)
	if target == "" {
		cfg, err := s.LoadConfig(ctx)
		if err != nil {
			return "", 0, err
		}
		target = ResolveSmartSidecarURL(cfg)
	}
	if target == "" {
		return "", 0, errors.New("sidecar url not configured")
	}
	if err := validateSmartConfig(&PrivacyMaskingConfig{SmartSidecarURL: target}); err != nil {
		return target, 0, err
	}
	if s.smart == nil {
		return target, 0, errors.New("smart detector not available")
	}
	checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	started := time.Now()
	err := s.smart.Health(checkCtx, target)
	return target, time.Since(started), err
}
