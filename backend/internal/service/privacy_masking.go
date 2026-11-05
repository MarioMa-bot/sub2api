package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// PrivacyMaskingAction 定义对检测到的敏感信息采取的动作
// Privacy Masking Action: defines the action to take for detected sensitive data
type PrivacyMaskingAction string

const (
	PrivacyMaskingActionMask  PrivacyMaskingAction = "mask"  // 脱敏
	PrivacyMaskingActionWarn  PrivacyMaskingAction = "warn"  // 警告但不脱敏
	PrivacyMaskingActionBlock PrivacyMaskingAction = "block" // 阻止请求
)

// IsValid 判断动作是否为受支持的取值
func (a PrivacyMaskingAction) IsValid() bool {
	switch a {
	case PrivacyMaskingActionMask, PrivacyMaskingActionWarn, PrivacyMaskingActionBlock:
		return true
	}
	return false
}

// PrivacyMaskingKind 敏感信息类型
// Privacy Masking Kind: types of sensitive information
type PrivacyMaskingKind string

const (
	KindAPIKey     PrivacyMaskingKind = "API_KEY"
	KindPrivateKey PrivacyMaskingKind = "PRIVATE_KEY"
	KindToken      PrivacyMaskingKind = "TOKEN"
	KindPassword   PrivacyMaskingKind = "PASSWORD"
	KindEmail      PrivacyMaskingKind = "EMAIL"
	KindPhone      PrivacyMaskingKind = "PHONE"
	KindCreditCard PrivacyMaskingKind = "CREDIT_CARD"
	KindIDNumber   PrivacyMaskingKind = "ID_NUMBER"
	KindCustom     PrivacyMaskingKind = "CUSTOM"
)

// privacyMaskingKindPattern 类型名必须能被占位符正则 {{KIND_hash}} 识别
var privacyMaskingKindPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,31}$`)

// PrivacyMaskingConfig 脱敏配置
// Privacy Masking Configuration: stores global or per-group masking settings
type PrivacyMaskingConfig struct {
	ID              int64                           `json:"id"`
	Enabled         bool                            `json:"enabled"`
	MaskSecrets     bool                            `json:"mask_secrets"`
	MaskPersonal    bool                            `json:"mask_personal"`
	MaskedKinds     map[string]bool                 `json:"masked_kinds"`
	CustomRules     []PrivacyMaskingRule            `json:"custom_rules"`
	AllowList       []string                        `json:"allow_list"`
	KindActions     map[string]PrivacyMaskingAction `json:"kind_actions"`
	GroupID         *int64                          `json:"group_id,omitempty"`
	CreatedAt       time.Time                       `json:"created_at"`
	UpdatedAt       time.Time                       `json:"updated_at"`
	UpdatedByUserID *int64                          `json:"updated_by_user_id,omitempty"`

	// 智能脱敏（Presidio 旁路），动作沿用 KindActions，MaskedKinds 不影响智能实体
	SmartMaskingEnabled bool     `json:"smart_masking_enabled"`
	SmartSidecarURL     string   `json:"smart_sidecar_url"`
	SmartTimeoutMS      int      `json:"smart_timeout_ms"`
	SmartMinScore       float64  `json:"smart_min_score"`
	SmartEntities       []string `json:"smart_entities"`
}

// PrivacyMaskingRule 自定义脱敏规则
type PrivacyMaskingRule struct {
	Pattern string               `json:"pattern"` // 正则表达式
	Kind    PrivacyMaskingKind   `json:"kind"`    // 类型
	Action  PrivacyMaskingAction `json:"action"`  // 动作
}

// PrivacyMaskingLog 脱敏日志（仅元数据，绝不包含原始敏感值）
type PrivacyMaskingLog struct {
	ID             int64                           `json:"id"`
	RequestID      string                          `json:"request_id,omitempty"`
	UserID         int64                           `json:"user_id"`
	APIKeyID       *int64                          `json:"api_key_id,omitempty"`
	GroupID        *int64                          `json:"group_id,omitempty"`
	DetectedAt     time.Time                       `json:"detected_at"`
	Kinds          map[string]int                  `json:"kinds"`
	Actions        map[string]PrivacyMaskingAction `json:"actions"`
	TotalCount     int                             `json:"total_count"`
	MaskedCount    int                             `json:"masked_count"`
	WarnedCount    int                             `json:"warned_count"`
	BlockedCount   int                             `json:"blocked_count"`
	RequestBlocked bool                            `json:"request_blocked"`
	ValueSamples   []PrivacyMaskingValueSample     `json:"value_samples,omitempty"`
	Endpoint       string                          `json:"endpoint,omitempty"`
	Model          string                          `json:"model,omitempty"`
	CreatedAt      time.Time                       `json:"created_at"`
}

// PrivacyMaskingValueSample 值的示例（不存储完整值）
type PrivacyMaskingValueSample struct {
	Kind   PrivacyMaskingKind `json:"kind"`
	Prefix string             `json:"prefix"` // 前几个字符，仅对带公开前缀的类型保留
	Length int                `json:"length"`
}

// PrivacyMaskingLogFilter 管理后台日志查询条件
type PrivacyMaskingLogFilter struct {
	Page      int
	PageSize  int
	Endpoint  string
	Kind      string
	Model     string
	UserID    *int64
	APIKeyID  *int64
	GroupID   *int64
	Blocked   *bool
	StartTime *time.Time
	EndTime   *time.Time
}

// PrivacyMaskingEndpointSummary 按 API 接口聚合的脱敏统计
type PrivacyMaskingEndpointSummary struct {
	Endpoint      string    `json:"endpoint"`
	EventCount    int64     `json:"event_count"`
	BlockedCount  int64     `json:"blocked_count"`
	DetectedCount int64     `json:"detected_count"`
	MaskedCount   int64     `json:"masked_count"`
	WarnedCount   int64     `json:"warned_count"`
	LastSeenAt    time.Time `json:"last_seen_at"`
}

// PrivacyMaskingStats 脱敏统计概览
type PrivacyMaskingStats struct {
	Since         time.Time        `json:"since"`
	EventCount    int64            `json:"event_count"`
	BlockedCount  int64            `json:"blocked_count"`
	DetectedCount int64            `json:"detected_count"`
	MaskedCount   int64            `json:"masked_count"`
	WarnedCount   int64            `json:"warned_count"`
	ByKind        map[string]int64 `json:"by_kind"`
}

// PrivacyMaskingStore 脱敏配置与日志的持久化接口（由 repository 实现）
type PrivacyMaskingStore interface {
	GetGlobalConfig(ctx context.Context) (*PrivacyMaskingConfig, error)
	SaveGlobalConfig(ctx context.Context, cfg *PrivacyMaskingConfig) error
	InsertLog(ctx context.Context, log *PrivacyMaskingLog) error
	ListLogs(ctx context.Context, filter PrivacyMaskingLogFilter) ([]PrivacyMaskingLog, int64, error)
	ListEndpointSummary(ctx context.Context, start, end *time.Time) ([]PrivacyMaskingEndpointSummary, error)
	GetStats(ctx context.Context, since time.Time) (*PrivacyMaskingStats, error)
}

// ErrPrivacyMaskingInvalidConfig 配置校验失败
var ErrPrivacyMaskingInvalidConfig = errors.New("invalid privacy masking config")

// ErrPrivacyMaskingStoreUnavailable 未配置持久化存储
var ErrPrivacyMaskingStoreUnavailable = errors.New("privacy masking store unavailable")

// privacyMaskingConfigTTL 配置缓存时间；管理员保存后会立即失效，TTL 只用于多实例间的最终一致
const privacyMaskingConfigTTL = 10 * time.Second

// placeholderPattern 占位符格式 {{KIND_hash}}
var placeholderPattern = regexp.MustCompile(`\{\{([A-Z][A-Z0-9_]*)_([a-z0-9]{8})\}\}`)

// PrivacyMaskingService 隐私脱敏服务
type PrivacyMaskingService struct {
	store   PrivacyMaskingStore
	smart   SmartDetector
	hmacKey []byte // HMAC key for generating stable placeholders

	mu          sync.RWMutex
	config      *PrivacyMaskingConfig
	rules       []*maskingRule
	configCache time.Time
}

// maskingRule 编译后的脱敏规则
type maskingRule struct {
	Kind    PrivacyMaskingKind
	Pattern *regexp.Regexp
	Action  PrivacyMaskingAction
	// base64Guard 跳过紧邻 base64 字符的命中
	base64Guard bool
}

// MaskingDetection 检测结果
type MaskingDetection struct {
	Kind   PrivacyMaskingKind
	Value  string
	Start  int
	End    int
	Action PrivacyMaskingAction
}

// MaskingResult 脱敏结果
type MaskingResult struct {
	Masked       string
	Detections   []MaskingDetection
	ShouldBlock  bool
	HasWarnings  bool
	MaskedCount  int
	WarnedCount  int
	BlockedCount int
	// Placeholders 占位符 -> 原始值，仅在本次请求生命周期内存在于内存中
	Placeholders map[string]string
}

// NewPrivacyMaskingService 创建脱敏服务
// Create a new privacy masking service instance
func NewPrivacyMaskingService(store PrivacyMaskingStore) (*PrivacyMaskingService, error) {
	// HMAC key 每个进程随机生成：占位符只需在单次请求内稳定
	hmacKey := make([]byte, 32)
	if _, err := rand.Read(hmacKey); err != nil {
		return nil, fmt.Errorf("failed to generate HMAC key: %w", err)
	}
	return &PrivacyMaskingService{store: store, smart: NewPresidioSmartDetector(), hmacKey: hmacKey}, nil
}

func defaultPrivacyMaskingConfig() *PrivacyMaskingConfig {
	return &PrivacyMaskingConfig{
		Enabled:     false,
		MaskSecrets: true,
		MaskedKinds: map[string]bool{},
		CustomRules: []PrivacyMaskingRule{},
		AllowList:   []string{},
		KindActions: map[string]PrivacyMaskingAction{},

		SmartTimeoutMS: DefaultSmartTimeoutMS,
		SmartMinScore:  DefaultSmartMinScore,
		SmartEntities:  DefaultSmartEntities(),
	}
}

// LoadConfig 加载全局配置（带短 TTL 缓存）
// 数据库暂时不可用时沿用上一次成功加载的配置；从未加载成功则返回错误，由调用方阻断请求。
func (s *PrivacyMaskingService) LoadConfig(ctx context.Context) (*PrivacyMaskingConfig, error) {
	cfg, _, err := s.snapshot(ctx)
	return cfg, err
}

// snapshot 返回一致的配置与已编译规则
func (s *PrivacyMaskingService) snapshot(ctx context.Context) (*PrivacyMaskingConfig, []*maskingRule, error) {
	s.mu.RLock()
	cfg, rules, loadedAt := s.config, s.rules, s.configCache
	s.mu.RUnlock()

	if cfg != nil && (s.store == nil || time.Since(loadedAt) < privacyMaskingConfigTTL) {
		return cfg, rules, nil
	}
	if s.store == nil {
		return defaultPrivacyMaskingConfig(), nil, nil
	}

	loaded, err := s.store.GetGlobalConfig(ctx)
	if err != nil || loaded == nil {
		if cfg != nil {
			return cfg, rules, nil
		}
		if err == nil {
			err = errors.New("privacy masking config not found")
		}
		return nil, nil, fmt.Errorf("load privacy masking config: %w", err)
	}

	compiled := compileMaskingRules(loaded)
	s.mu.Lock()
	s.config, s.rules, s.configCache = loaded, compiled, time.Now()
	s.mu.Unlock()
	return loaded, compiled, nil
}

// applyConfig 直接设置配置（测试与保存后使用）
func (s *PrivacyMaskingService) applyConfig(cfg *PrivacyMaskingConfig) {
	compiled := compileMaskingRules(cfg)
	s.mu.Lock()
	s.config, s.rules, s.configCache = cfg, compiled, time.Now()
	s.mu.Unlock()
}

// InvalidateConfig 让下一次请求重新从数据库加载配置
func (s *PrivacyMaskingService) InvalidateConfig() {
	s.mu.Lock()
	s.configCache = time.Time{}
	s.mu.Unlock()
}

// compileMaskingRules 编译所有规则
func compileMaskingRules(cfg *PrivacyMaskingConfig) []*maskingRule {
	if cfg == nil {
		return nil
	}
	rules := make([]*maskingRule, 0)
	actionFor := func(kind PrivacyMaskingKind) PrivacyMaskingAction {
		if action, ok := cfg.KindActions[string(kind)]; ok && action.IsValid() {
			return action
		}
		return PrivacyMaskingActionMask
	}
	// MaskedKinds 中显式设为 false 的类型不检测
	kindEnabled := func(kind PrivacyMaskingKind) bool {
		enabled, ok := cfg.MaskedKinds[string(kind)]
		return !ok || enabled
	}
	add := func(p builtinRule) {
		if kindEnabled(p.kind) {
			rules = append(rules, &maskingRule{Kind: p.kind, Pattern: p.pattern, Action: actionFor(p.kind), base64Guard: p.base64Guard})
		}
	}

	if cfg.MaskSecrets {
		for _, p := range builtinSecretRules {
			add(p)
		}
	}
	if cfg.MaskPersonal {
		for _, p := range builtinPersonalRules {
			add(p)
		}
	}
	for _, customRule := range cfg.CustomRules {
		pattern, err := regexp.Compile(customRule.Pattern)
		if err != nil {
			continue
		}
		kind := customRule.Kind
		if !privacyMaskingKindPattern.MatchString(string(kind)) {
			kind = KindCustom
		}
		action := customRule.Action
		if !action.IsValid() {
			action = actionFor(kind)
		}
		rules = append(rules, &maskingRule{Kind: kind, Pattern: pattern, Action: action})
	}
	return rules
}

type builtinRule struct {
	kind    PrivacyMaskingKind
	pattern *regexp.Regexp
	// base64Guard 命中片段紧邻 + / = 时视为 base64 数据（如图片）的一段而跳过
	base64Guard bool
}

func isBase64Neighbor(text string, start, end int) bool {
	isB64 := func(ch byte) bool { return ch == '+' || ch == '/' || ch == '=' }
	return (start > 0 && isB64(text[start-1])) || (end < len(text) && isB64(text[end]))
}

// 顺序即优先级：同一片段被多条规则命中时保留先出现的（具体规则在前，通用规则在后）
var builtinSecretRules = []builtinRule{
	{kind: KindPrivateKey, pattern: regexp.MustCompile(`-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----[\s\S]+?-----END (?:RSA |EC |OPENSSH )?PRIVATE KEY-----`)},
	{kind: KindAPIKey, pattern: regexp.MustCompile(`\b(sk-ant-[a-zA-Z0-9_-]{20,})`)},                           // Anthropic style
	{kind: KindAPIKey, pattern: regexp.MustCompile(`\b(sk-[a-zA-Z0-9_-]{20,})`)},                               // OpenAI style
	{kind: KindToken, pattern: regexp.MustCompile(`\b(gh[pousr]_[a-zA-Z0-9]{36,})\b`)},                         // GitHub tokens
	{kind: KindToken, pattern: regexp.MustCompile(`\b(eyJ[a-zA-Z0-9_-]+\.eyJ[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+)`)}, // JWT
	{kind: KindPassword, pattern: regexp.MustCompile(`(?i)(?:password|pwd|passwd)\s*[:=]\s*['"]?([^'"\s\\]{8,})`)},
	{kind: KindAPIKey, pattern: regexp.MustCompile(`\b([a-zA-Z0-9]{32,})\b`), base64Guard: true}, // Generic long keys
}

var builtinPersonalRules = []builtinRule{
	{kind: KindEmail, pattern: regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b`)},
	{kind: KindPhone, pattern: regexp.MustCompile(`\b(?:\+?1[-.]?)?\(?\d{3}\)?[-.]?\d{3}[-.]?\d{4}\b`)}, // US phone
	{kind: KindPhone, pattern: regexp.MustCompile(`(?:\+86)?\b1[3-9]\d{9}\b`)},                          // China phone
	{kind: KindCreditCard, pattern: regexp.MustCompile(`\b\d{4}[-\s]?\d{4}[-\s]?\d{4}[-\s]?\d{4}\b`)},
}

// ValidatePrivacyMaskingConfig 校验管理员提交的配置
func ValidatePrivacyMaskingConfig(cfg *PrivacyMaskingConfig) error {
	if cfg == nil {
		return fmt.Errorf("%w: config is required", ErrPrivacyMaskingInvalidConfig)
	}
	for kind, action := range cfg.KindActions {
		if !action.IsValid() {
			return fmt.Errorf("%w: invalid action %q for kind %s", ErrPrivacyMaskingInvalidConfig, action, kind)
		}
	}
	for i, rule := range cfg.CustomRules {
		if strings.TrimSpace(rule.Pattern) == "" {
			return fmt.Errorf("%w: custom rule #%d has empty pattern", ErrPrivacyMaskingInvalidConfig, i+1)
		}
		if _, err := regexp.Compile(rule.Pattern); err != nil {
			return fmt.Errorf("%w: custom rule #%d: %v", ErrPrivacyMaskingInvalidConfig, i+1, err)
		}
		if rule.Kind != "" && !privacyMaskingKindPattern.MatchString(string(rule.Kind)) {
			return fmt.Errorf("%w: custom rule #%d kind must match [A-Z][A-Z0-9_]*", ErrPrivacyMaskingInvalidConfig, i+1)
		}
		if rule.Action != "" && !rule.Action.IsValid() {
			return fmt.Errorf("%w: custom rule #%d has invalid action %q", ErrPrivacyMaskingInvalidConfig, i+1, rule.Action)
		}
	}
	return validateSmartConfig(cfg)
}

// Mask 对文本进行脱敏（仅规则，不调用智能检测旁路）
func (s *PrivacyMaskingService) Mask(ctx context.Context, text string) (*MaskingResult, error) {
	cfg, rules, err := s.snapshot(ctx)
	if err != nil {
		return nil, err
	}
	if !cfg.Enabled {
		return &MaskingResult{Masked: text}, nil
	}
	return s.buildMaskingResult(text, detectWithRules(cfg, rules, text)), nil
}

func detectWithRules(cfg *PrivacyMaskingConfig, rules []*maskingRule, text string) []MaskingDetection {
	detections := make([]MaskingDetection, 0)
	for _, rule := range rules {
		for _, match := range rule.Pattern.FindAllStringSubmatchIndex(text, -1) {
			start, end := match[0], match[1]
			// 如果有捕获组，使用第一个捕获组
			if len(match) >= 4 && match[2] >= 0 {
				start, end = match[2], match[3]
			}
			if start >= end || (rule.base64Guard && isBase64Neighbor(text, start, end)) {
				continue
			}
			value := text[start:end]
			if isAllowedValue(cfg, value) {
				continue
			}
			detections = append(detections, MaskingDetection{
				Kind: rule.Kind, Value: value, Start: start, End: end, Action: rule.Action,
			})
		}
	}
	return detections
}

// buildMaskingResult 合并、去重并应用检测结果；规则命中排在智能命中之前，重叠时优先保留规则结果
func (s *PrivacyMaskingService) buildMaskingResult(text string, detections []MaskingDetection) *MaskingResult {
	result := &MaskingResult{Masked: text, Detections: dedupeDetections(detections)}
	if len(result.Detections) > 0 {
		result.Masked = s.applyMasking(text, result)
	}
	return result
}

// dedupeDetections 去掉重叠的命中：多条规则命中同一片段时只保留最先出现、最长的一条，
// 否则从后往前替换会破坏文本。规则顺序决定优先级（具体规则排在通用规则之前）。
func dedupeDetections(detections []MaskingDetection) []MaskingDetection {
	if len(detections) < 2 {
		return detections
	}
	sort.SliceStable(detections, func(i, j int) bool {
		if detections[i].Start != detections[j].Start {
			return detections[i].Start < detections[j].Start
		}
		return detections[i].End > detections[j].End
	})
	kept := detections[:0:0]
	lastEnd := -1
	for _, det := range detections {
		if det.Start < lastEnd {
			continue
		}
		kept = append(kept, det)
		lastEnd = det.End
	}
	return kept
}

// applyMasking 应用脱敏（detections 已按位置升序且不重叠）
func (s *PrivacyMaskingService) applyMasking(text string, result *MaskingResult) string {
	var b strings.Builder
	b.Grow(len(text))
	cursor := 0
	for _, det := range result.Detections {
		switch det.Action {
		case PrivacyMaskingActionWarn:
			result.HasWarnings = true
			result.WarnedCount++
			continue
		case PrivacyMaskingActionBlock:
			result.ShouldBlock = true
			result.BlockedCount++
		default:
			result.MaskedCount++
		}
		placeholder := s.generatePlaceholder(det.Kind, det.Value)
		if result.Placeholders == nil {
			result.Placeholders = make(map[string]string)
		}
		result.Placeholders[placeholder] = det.Value
		_, _ = b.WriteString(text[cursor:det.Start])
		_, _ = b.WriteString(placeholder)
		cursor = det.End
	}
	_, _ = b.WriteString(text[cursor:])
	return b.String()
}

// generatePlaceholder 生成稳定的占位符
// Generate a stable placeholder for a sensitive value using HMAC-SHA256
func (s *PrivacyMaskingService) generatePlaceholder(kind PrivacyMaskingKind, value string) string {
	h := hmac.New(sha256.New, s.hmacKey)
	_, _ = h.Write([]byte(value))
	suffix := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(h.Sum(nil)[:8])
	return fmt.Sprintf("{{%s_%s}}", kind, strings.ToLower(suffix)[:8])
}

// UnmaskWith 使用本次请求的占位符映射恢复原始值
func UnmaskWith(placeholders map[string]string, text string) string {
	if len(placeholders) == 0 || !strings.Contains(text, "{{") {
		return text
	}
	return placeholderPattern.ReplaceAllStringFunc(text, func(ph string) string {
		if original, ok := placeholders[ph]; ok {
			return original
		}
		return ph
	})
}

// isAllowedValue 检查值是否在白名单中（精确匹配，不区分大小写）
func isAllowedValue(cfg *PrivacyMaskingConfig, value string) bool {
	for _, allowed := range cfg.AllowList {
		if strings.EqualFold(allowed, value) {
			return true
		}
	}
	return false
}

// MaskJSON 对 JSON 进行脱敏；脱敏后不再是合法 JSON 时返回错误（失败即阻断，绝不放行原文）
// 启用智能检测时，L1 规则与 L2 旁路结果合并；旁路失败只会少了 L2 结果，不影响请求。
func (s *PrivacyMaskingService) MaskJSON(ctx context.Context, data []byte) ([]byte, *MaskingResult, error) {
	cfg, rules, err := s.snapshot(ctx)
	if err != nil {
		return nil, nil, err
	}
	if !cfg.Enabled {
		return data, &MaskingResult{Masked: string(data)}, nil
	}
	text := string(data)
	detections := detectWithRules(cfg, rules, text)
	if cfg.SmartMaskingEnabled {
		detections = append(detections, s.detectSmart(ctx, cfg, data)...)
	}
	result := s.buildMaskingResult(text, detections)
	if len(result.Placeholders) == 0 {
		return data, result, nil
	}
	masked := []byte(result.Masked)
	if !json.Valid(masked) {
		return nil, result, errors.New("masked body is not valid JSON")
	}
	return masked, result, nil
}

// maskingSamplePrefixKinds 只有这些类型的前缀是公开的格式标识（如 sk-、ghp_、eyJ），可安全保留
var maskingSamplePrefixKinds = map[PrivacyMaskingKind]bool{
	KindAPIKey: true,
	KindToken:  true,
}

// CreateSamplePrefix 创建值的安全前缀示例
func CreateSamplePrefix(kind PrivacyMaskingKind, value string, maxLen int) string {
	if !maskingSamplePrefixKinds[kind] || len(value) < 16 {
		return ""
	}
	if maxLen <= 0 || maxLen > 4 {
		maxLen = 4
	}
	return value[:maxLen] + "..."
}

// BuildPrivacyMaskingLog 由脱敏结果生成仅含元数据的日志记录
func BuildPrivacyMaskingLog(result *MaskingResult) *PrivacyMaskingLog {
	if result == nil || len(result.Detections) == 0 {
		return nil
	}
	kinds := make(map[string]int)
	actions := make(map[string]PrivacyMaskingAction)
	samples := make([]PrivacyMaskingValueSample, 0, 10)
	for _, det := range result.Detections {
		kind := string(det.Kind)
		kinds[kind]++
		// 同一类型出现多种动作时记录最严格的那个
		if actionSeverity(det.Action) > actionSeverity(actions[kind]) {
			actions[kind] = det.Action
		}
		if len(samples) < 10 {
			samples = append(samples, PrivacyMaskingValueSample{
				Kind:   det.Kind,
				Prefix: CreateSamplePrefix(det.Kind, det.Value, 4),
				Length: len(det.Value),
			})
		}
	}
	return &PrivacyMaskingLog{
		DetectedAt:     time.Now(),
		Kinds:          kinds,
		Actions:        actions,
		TotalCount:     len(result.Detections),
		MaskedCount:    result.MaskedCount,
		WarnedCount:    result.WarnedCount,
		BlockedCount:   result.BlockedCount,
		RequestBlocked: result.ShouldBlock,
		ValueSamples:   samples,
	}
}

func actionSeverity(a PrivacyMaskingAction) int {
	switch a {
	case PrivacyMaskingActionBlock:
		return 3
	case PrivacyMaskingActionMask:
		return 2
	case PrivacyMaskingActionWarn:
		return 1
	}
	return 0
}

// LogMasking 记录脱敏事件
func (s *PrivacyMaskingService) LogMasking(ctx context.Context, log *PrivacyMaskingLog) error {
	if log == nil {
		return nil
	}
	if s.store == nil {
		return ErrPrivacyMaskingStoreUnavailable
	}
	return s.store.InsertLog(ctx, log)
}

// GetConfig 管理后台读取全局配置（不走缓存）
func (s *PrivacyMaskingService) GetConfig(ctx context.Context) (*PrivacyMaskingConfig, error) {
	if s.store == nil {
		return nil, ErrPrivacyMaskingStoreUnavailable
	}
	return s.store.GetGlobalConfig(ctx)
}

// SaveConfig 校验并保存全局配置，保存成功后本实例立即生效
func (s *PrivacyMaskingService) SaveConfig(ctx context.Context, cfg *PrivacyMaskingConfig) error {
	if err := ValidatePrivacyMaskingConfig(cfg); err != nil {
		return err
	}
	if s.store == nil {
		return ErrPrivacyMaskingStoreUnavailable
	}
	if cfg.MaskedKinds == nil {
		cfg.MaskedKinds = map[string]bool{}
	}
	if cfg.CustomRules == nil {
		cfg.CustomRules = []PrivacyMaskingRule{}
	}
	if cfg.AllowList == nil {
		cfg.AllowList = []string{}
	}
	if cfg.KindActions == nil {
		cfg.KindActions = map[string]PrivacyMaskingAction{}
	}
	cfg.SmartSidecarURL = strings.TrimSpace(cfg.SmartSidecarURL)
	if cfg.SmartTimeoutMS == 0 {
		cfg.SmartTimeoutMS = DefaultSmartTimeoutMS
	}
	if cfg.SmartEntities == nil {
		cfg.SmartEntities = []string{}
	}
	for i, kind := range cfg.SmartEntities {
		cfg.SmartEntities[i] = normalizeSmartKind(kind)
	}
	if err := s.store.SaveGlobalConfig(ctx, cfg); err != nil {
		return err
	}
	s.InvalidateConfig()
	return nil
}

// ListLogs 管理后台分页查询脱敏日志
func (s *PrivacyMaskingService) ListLogs(ctx context.Context, filter PrivacyMaskingLogFilter) ([]PrivacyMaskingLog, int64, error) {
	if s.store == nil {
		return nil, 0, ErrPrivacyMaskingStoreUnavailable
	}
	return s.store.ListLogs(ctx, filter)
}

// ListEndpointSummary 管理后台按 API 接口聚合脱敏事件
func (s *PrivacyMaskingService) ListEndpointSummary(ctx context.Context, start, end *time.Time) ([]PrivacyMaskingEndpointSummary, error) {
	if s.store == nil {
		return nil, ErrPrivacyMaskingStoreUnavailable
	}
	return s.store.ListEndpointSummary(ctx, start, end)
}

// GetStats 管理后台统计概览
func (s *PrivacyMaskingService) GetStats(ctx context.Context, since time.Time) (*PrivacyMaskingStats, error) {
	if s.store == nil {
		return nil, ErrPrivacyMaskingStoreUnavailable
	}
	return s.store.GetStats(ctx, since)
}
