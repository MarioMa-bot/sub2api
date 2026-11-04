package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"regexp"
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

// PrivacyMaskingKind 敏感信息类型
// Privacy Masking Kind: types of sensitive information
type PrivacyMaskingKind string

const (
	KindAPIKey       PrivacyMaskingKind = "API_KEY"
	KindPrivateKey   PrivacyMaskingKind = "PRIVATE_KEY"
	KindToken        PrivacyMaskingKind = "TOKEN"
	KindPassword     PrivacyMaskingKind = "PASSWORD"
	KindEmail        PrivacyMaskingKind = "EMAIL"
	KindPhone        PrivacyMaskingKind = "PHONE"
	KindCreditCard   PrivacyMaskingKind = "CREDIT_CARD"
	KindIDNumber     PrivacyMaskingKind = "ID_NUMBER"
	KindCustom       PrivacyMaskingKind = "CUSTOM"
)

// PrivacyMaskingConfig 脱敏配置
// Privacy Masking Configuration: stores global or per-group masking settings
type PrivacyMaskingConfig struct {
	ID             int64                               `json:"id" gorm:"primaryKey"`
	Enabled        bool                                `json:"enabled" gorm:"not null;default:false"`
	MaskSecrets    bool                                `json:"mask_secrets" gorm:"not null;default:true"`
	MaskPersonal   bool                                `json:"mask_personal" gorm:"not null;default:false"`
	MaskedKinds    map[string]bool                     `json:"masked_kinds" gorm:"type:jsonb;not null;default:'{}'"`
	CustomRules    []PrivacyMaskingRule                `json:"custom_rules" gorm:"type:jsonb;not null;default:'[]'"`
	AllowList      []string                            `json:"allow_list" gorm:"type:jsonb;not null;default:'[]'"`
	KindActions    map[string]PrivacyMaskingAction     `json:"kind_actions" gorm:"type:jsonb;not null;default:'{}'"`
	GroupID        *int64                              `json:"group_id,omitempty" gorm:"uniqueIndex"`
	CreatedAt      time.Time                           `json:"created_at" gorm:"not null;default:now()"`
	UpdatedAt      time.Time                           `json:"updated_at" gorm:"not null;default:now()"`
	UpdatedByUserID *int64                             `json:"updated_by_user_id,omitempty"`
}

// PrivacyMaskingRule 自定义脱敏规则
type PrivacyMaskingRule struct {
	Pattern string                   `json:"pattern"` // 正则表达式
	Kind    PrivacyMaskingKind       `json:"kind"`    // 类型
	Action  PrivacyMaskingAction     `json:"action"`  // 动作
}

// PrivacyMaskingLog 脱敏日志
type PrivacyMaskingLog struct {
	ID             int64                           `json:"id" gorm:"primaryKey"`
	RequestID      string                          `json:"request_id,omitempty" gorm:"index"`
	UserID         int64                           `json:"user_id" gorm:"not null;index"`
	APIKeyID       *int64                          `json:"api_key_id,omitempty"`
	GroupID        *int64                          `json:"group_id,omitempty" gorm:"index"`
	DetectedAt     time.Time                       `json:"detected_at" gorm:"not null;default:now();index"`
	Kinds          map[string]int                  `json:"kinds" gorm:"type:jsonb;not null"`
	Actions        map[string]PrivacyMaskingAction `json:"actions" gorm:"type:jsonb;not null"`
	TotalCount     int                             `json:"total_count" gorm:"not null;default:0"`
	MaskedCount    int                             `json:"masked_count" gorm:"not null;default:0"`
	WarnedCount    int                             `json:"warned_count" gorm:"not null;default:0"`
	BlockedCount   int                             `json:"blocked_count" gorm:"not null;default:0"`
	RequestBlocked bool                            `json:"request_blocked" gorm:"not null;default:false;index"`
	ValueSamples   []PrivacyMaskingValueSample     `json:"value_samples,omitempty" gorm:"type:jsonb"`
	Endpoint       string                          `json:"endpoint,omitempty"`
	Model          string                          `json:"model,omitempty"`
	CreatedAt      time.Time                       `json:"created_at" gorm:"not null;default:now()"`
}

// PrivacyMaskingValueSample 值的示例（不存储完整值）
type PrivacyMaskingValueSample struct {
	Kind   PrivacyMaskingKind `json:"kind"`
	Prefix string             `json:"prefix"` // 前几个字符
	Length int                `json:"length"`
}

// TableName 指定表名
func (PrivacyMaskingConfig) TableName() string {
	return "privacy_masking_config"
}

func (PrivacyMaskingLog) TableName() string {
	return "privacy_masking_log"
}

// PrivacyMaskingService 隐私脱敏服务
type PrivacyMaskingService struct {
	mu          sync.RWMutex
	values      map[string]string // placeholder -> original value (仅在内存中)
	hmacKey     []byte            // HMAC key for generating stable placeholders
	config      *PrivacyMaskingConfig
	configCache time.Time
	rules       []*maskingRule
}

// maskingRule 编译后的脱敏规则
type maskingRule struct {
	Kind    PrivacyMaskingKind
	Pattern *regexp.Regexp
	Action  PrivacyMaskingAction
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
	Masked         string
	Detections     []MaskingDetection
	ShouldBlock    bool
	HasWarnings    bool
	MaskedCount    int
	WarnedCount    int
	BlockedCount   int
}

// NewPrivacyMaskingService 创建脱敏服务
// Create a new privacy masking service instance
func NewPrivacyMaskingService() (*PrivacyMaskingService, error) {
	// 生成 HMAC key（应该持久化，但为简单起见这里每次生成）
	// Generate HMAC key (should be persisted, but generated each time for simplicity)
	hmacKey := make([]byte, 32)
	if _, err := rand.Read(hmacKey); err != nil {
		return nil, fmt.Errorf("failed to generate HMAC key: %w", err)
	}

	s := &PrivacyMaskingService{
		values:  make(map[string]string),
		hmacKey: hmacKey,
	}

	return s, nil
}

// LoadConfig 加载配置（从数据库或缓存）
func (s *PrivacyMaskingService) LoadConfig(ctx context.Context) (*PrivacyMaskingConfig, error) {
	s.mu.RLock()
	if s.config != nil && time.Since(s.configCache) < 60*time.Second {
		defer s.mu.RUnlock()
		return s.config, nil
	}
	s.mu.RUnlock()

	// TODO: 从数据库加载配置
	// 这里返回默认配置作为示例
	config := &PrivacyMaskingConfig{
		Enabled:      false,
		MaskSecrets:  true,
		MaskPersonal: false,
		MaskedKinds:  make(map[string]bool),
		CustomRules:  []PrivacyMaskingRule{},
		AllowList:    []string{},
		KindActions:  make(map[string]PrivacyMaskingAction),
	}

	s.mu.Lock()
	s.config = config
	s.configCache = time.Now()
	s.compileRules()
	s.mu.Unlock()

	return config, nil
}

// compileRules 编译所有规则
func (s *PrivacyMaskingService) compileRules() {
	s.rules = make([]*maskingRule, 0)

	if s.config == nil {
		return
	}

	// 添加内置规则
	if s.config.MaskSecrets {
		s.addBuiltinSecretRules()
	}
	if s.config.MaskPersonal {
		s.addBuiltinPersonalRules()
	}

	// 添加自定义规则
	for _, customRule := range s.config.CustomRules {
		if pattern, err := regexp.Compile(customRule.Pattern); err == nil {
			s.rules = append(s.rules, &maskingRule{
				Kind:    customRule.Kind,
				Pattern: pattern,
				Action:  customRule.Action,
			})
		}
	}
}

// addBuiltinSecretRules 添加内置密钥规则
func (s *PrivacyMaskingService) addBuiltinSecretRules() {
	secretPatterns := []struct {
		kind    PrivacyMaskingKind
		pattern string
	}{
		{KindAPIKey, `\b(sk-[a-zA-Z0-9]{20,})\b`},                                                    // OpenAI style
		{KindAPIKey, `\b(sk-ant-[a-zA-Z0-9-]{20,})\b`},                                              // Anthropic style
		{KindAPIKey, `\b([a-zA-Z0-9]{32,})\b`},                                                      // Generic long keys
		{KindToken, `\b(ghp_[a-zA-Z0-9]{36,})\b`},                                                   // GitHub token
		{KindToken, `\b(gho_[a-zA-Z0-9]{36,})\b`},                                                   // GitHub OAuth
		{KindPrivateKey, `-----BEGIN (RSA |EC |OPENSSH )?PRIVATE KEY-----[\s\S]+?-----END \1PRIVATE KEY-----`},
		{KindPassword, `(?i)(password|pwd|passwd)\s*[:=]\s*['"]?([^'"\s]{8,})['"]?`},
		{KindToken, `\b(eyJ[a-zA-Z0-9_-]+\.eyJ[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+)\b`},                  // JWT
	}

	for _, p := range secretPatterns {
		if pattern, err := regexp.Compile(p.pattern); err == nil {
			action := s.getKindAction(p.kind)
			s.rules = append(s.rules, &maskingRule{
				Kind:    p.kind,
				Pattern: pattern,
				Action:  action,
			})
		}
	}
}

// addBuiltinPersonalRules 添加内置个人信息规则
func (s *PrivacyMaskingService) addBuiltinPersonalRules() {
	personalPatterns := []struct {
		kind    PrivacyMaskingKind
		pattern string
	}{
		{KindEmail, `\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`},
		{KindPhone, `\b(\+?1[-.]?)?\(?\d{3}\)?[-.]?\d{3}[-.]?\d{4}\b`},                            // US phone
		{KindPhone, `\b(\+86)?1[3-9]\d{9}\b`},                                                     // China phone
		{KindCreditCard, `\b\d{4}[-\s]?\d{4}[-\s]?\d{4}[-\s]?\d{4}\b`},
	}

	for _, p := range personalPatterns {
		if pattern, err := regexp.Compile(p.pattern); err == nil {
			action := s.getKindAction(p.kind)
			s.rules = append(s.rules, &maskingRule{
				Kind:    p.kind,
				Pattern: pattern,
				Action:  action,
			})
		}
	}
}

// getKindAction 获取某类型的动作
func (s *PrivacyMaskingService) getKindAction(kind PrivacyMaskingKind) PrivacyMaskingAction {
	if action, ok := s.config.KindActions[string(kind)]; ok {
		return action
	}
	return PrivacyMaskingActionMask // 默认脱敏
}

// Mask 对文本进行脱敏
func (s *PrivacyMaskingService) Mask(ctx context.Context, text string) (*MaskingResult, error) {
	config, err := s.LoadConfig(ctx)
	if err != nil {
		return nil, err
	}

	if !config.Enabled {
		return &MaskingResult{Masked: text}, nil
	}

	s.mu.RLock()
	rules := s.rules
	s.mu.RUnlock()

	result := &MaskingResult{
		Masked:      text,
		Detections:  make([]MaskingDetection, 0),
	}

	// 检测所有敏感信息
	for _, rule := range rules {
		matches := rule.Pattern.FindAllStringSubmatchIndex(text, -1)
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}

			start, end := match[0], match[1]
			// 如果有捕获组，使用第一个捕获组
			if len(match) >= 4 {
				start, end = match[2], match[3]
			}

			value := text[start:end]
			
			// 检查白名单
			if s.isAllowed(value) {
				continue
			}

			result.Detections = append(result.Detections, MaskingDetection{
				Kind:   rule.Kind,
				Value:  value,
				Start:  start,
				End:    end,
				Action: rule.Action,
			})
		}
	}

	// 应用脱敏
	if len(result.Detections) > 0 {
		result.Masked = s.applyMasking(text, result.Detections, result)
	}

	return result, nil
}

// applyMasking 应用脱敏（从后往前替换，避免位置偏移）
func (s *PrivacyMaskingService) applyMasking(text string, detections []MaskingDetection, result *MaskingResult) string {
	// 按位置从后往前排序
	sorted := make([]MaskingDetection, len(detections))
	copy(sorted, detections)
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i].Start < sorted[j].Start {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	masked := text
	for _, det := range sorted {
		switch det.Action {
		case PrivacyMaskingActionMask:
			placeholder := s.generatePlaceholder(det.Kind, det.Value)
			masked = masked[:det.Start] + placeholder + masked[det.End:]
			result.MaskedCount++
		case PrivacyMaskingActionWarn:
			result.HasWarnings = true
			result.WarnedCount++
		case PrivacyMaskingActionBlock:
			result.ShouldBlock = true
			result.BlockedCount++
			// 仍然脱敏（请求会被阻止）
			placeholder := s.generatePlaceholder(det.Kind, det.Value)
			masked = masked[:det.Start] + placeholder + masked[det.End:]
		}
	}

	return masked
}

// generatePlaceholder 生成稳定的占位符
// Generate a stable placeholder for a sensitive value using HMAC-SHA256
func (s *PrivacyMaskingService) generatePlaceholder(kind PrivacyMaskingKind, value string) string {
	// 使用 HMAC 生成稳定的哈希
	// Use HMAC to generate a stable hash
	h := hmac.New(sha256.New, s.hmacKey)
	h.Write([]byte(value))
	hash := h.Sum(nil)
	
	// 取前8字节，转为 base32（更短更可读）
	// Take first 8 bytes and convert to base32 (shorter and more readable)
	suffix := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(hash[:8])
	suffix = strings.ToLower(suffix)[:8]
	
	placeholder := fmt.Sprintf("{{%s_%s}}", kind, suffix)
	
	// 存储映射（仅在内存中）
	// Store mapping (in memory only, never persisted)
	s.mu.Lock()
	s.values[placeholder] = value
	s.mu.Unlock()
	
	return placeholder
}

// Unmask 恢复占位符为原始值
// Restore placeholders back to original values in response text
func (s *PrivacyMaskingService) Unmask(ctx context.Context, text string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := text
	// 查找所有占位符 {{KIND_hash}}
	// Find all placeholders in the format {{KIND_hash}}
	re := regexp.MustCompile(`\{\{([A-Z_]+)_([a-z0-9]+)\}\}`)
	matches := re.FindAllStringSubmatch(text, -1)

	for _, match := range matches {
		if len(match) >= 1 {
			placeholder := match[0]
			if original, ok := s.values[placeholder]; ok {
				result = strings.ReplaceAll(result, placeholder, original)
			}
		}
	}

	return result
}

// isAllowed 检查值是否在白名单中
func (s *PrivacyMaskingService) isAllowed(value string) bool {
	if s.config == nil {
		return false
	}

	valueLower := strings.ToLower(value)
	for _, allowed := range s.config.AllowList {
		if strings.ToLower(allowed) == valueLower {
			return true
		}
	}
	return false
}

// MaskJSON 对 JSON 进行脱敏
func (s *PrivacyMaskingService) MaskJSON(ctx context.Context, data []byte) ([]byte, *MaskingResult, error) {
	// 简单实现：将 JSON 转为字符串脱敏，再转回
	// 生产环境应该遍历 JSON 树进行脱敏
	text := string(data)
	result, err := s.Mask(ctx, text)
	if err != nil {
		return nil, nil, err
	}

	// 验证结果仍然是有效的 JSON
	var test interface{}
	if err := json.Unmarshal([]byte(result.Masked), &test); err != nil {
		// 如果脱敏后 JSON 无效，返回原始数据
		return data, &MaskingResult{Masked: text}, nil
	}

	return []byte(result.Masked), result, nil
}

// UnmaskJSON 恢复 JSON 中的占位符
func (s *PrivacyMaskingService) UnmaskJSON(ctx context.Context, data []byte) []byte {
	text := string(data)
	unmasked := s.Unmask(ctx, text)
	return []byte(unmasked)
}

// CreateSamplePrefix 创建值的安全前缀示例
func CreateSamplePrefix(value string, maxLen int) string {
	if maxLen <= 0 {
		maxLen = 4
	}
	if len(value) <= maxLen {
		return value[:len(value)/2] + "..."
	}
	return value[:maxLen] + "..."
}

// LogMasking 记录脱敏事件
func (s *PrivacyMaskingService) LogMasking(ctx context.Context, log *PrivacyMaskingLog) error {
	// TODO: 实现日志记录到数据库
	// 这里只是接口定义
	return nil
}
