package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// PrivacyMaskingMiddleware 隐私脱敏中间件
type PrivacyMaskingMiddleware struct {
	service *PrivacyMaskingService
	logger  *zap.Logger
}

// NewPrivacyMaskingMiddleware 创建中间件实例
func NewPrivacyMaskingMiddleware(service *PrivacyMaskingService, logger *zap.Logger) *PrivacyMaskingMiddleware {
	return &PrivacyMaskingMiddleware{
		service: service,
		logger:  logger,
	}
}

// contextKey 用于存储脱敏上下文的键
type contextKey string

const (
	maskingContextKey contextKey = "privacy_masking_context"
)

// maskingContext 脱敏上下文（在请求生命周期中传递）
type maskingContext struct {
	enabled      bool
	shouldBlock  bool
	result       *MaskingResult
}

// MaskRequest 脱敏请求中间件
func (m *PrivacyMaskingMiddleware) MaskRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查是否启用脱敏
		config, err := m.service.LoadConfig(c.Request.Context())
		if err != nil {
			m.logger.Error("Failed to load masking config", zap.Error(err))
			c.Next()
			return
		}

		if !config.Enabled {
			c.Next()
			return
		}

		// 只处理 POST/PUT/PATCH 请求
		if c.Request.Method != http.MethodPost && 
		   c.Request.Method != http.MethodPut && 
		   c.Request.Method != http.MethodPatch {
			c.Next()
			return
		}

		// 读取请求体
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			m.logger.Error("Failed to read request body for masking", zap.Error(err))
			c.Next()
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		// 检测并脱敏
		maskedBody, result, err := m.service.MaskJSON(c.Request.Context(), bodyBytes)
		if err != nil {
			m.logger.Error("Failed to mask request body", zap.Error(err))
			// 如果脱敏失败，阻止请求（安全优先）
			c.JSON(http.StatusForbidden, gin.H{
				"error": gin.H{
					"type":    "privacy_masking_error",
					"message": "Request masking failed for security reasons",
				},
			})
			c.Abort()
			return
		}

		// 如果需要阻止请求
		if result.ShouldBlock {
			m.logger.Warn("Request blocked by privacy masking",
				zap.Int("blocked_count", result.BlockedCount),
				zap.Strings("detections", extractKinds(result.Detections)))

			c.JSON(http.StatusForbidden, gin.H{
				"error": gin.H{
					"type":    "privacy_masking_blocked",
					"message": "Request contains blocked sensitive information",
				},
			})
			c.Abort()
			return
		}

		// 如果有警告
		if result.HasWarnings {
			m.logger.Warn("Request contains sensitive information (warning only)",
				zap.Int("warned_count", result.WarnedCount),
				zap.Strings("detections", extractKinds(result.Detections)))
		}

		// 替换请求体
		if result.MaskedCount > 0 {
			c.Request.Body = io.NopCloser(bytes.NewBuffer(maskedBody))
			c.Request.ContentLength = int64(len(maskedBody))
			m.logger.Debug("Request body masked",
				zap.Int("masked_count", result.MaskedCount),
				zap.Strings("kinds", extractKinds(result.Detections)))
		}

		// 存储脱敏上下文
		ctx := &maskingContext{
			enabled:     true,
			shouldBlock: result.ShouldBlock,
			result:      result,
		}
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), maskingContextKey, ctx))

		c.Next()
	}
}

// UnmaskResponse 恢复响应中间件（需要在写入响应前调用）
type unmaskingResponseWriter struct {
	gin.ResponseWriter
	service *PrivacyMaskingService
	ctx     context.Context
	body    *bytes.Buffer
	logger  *zap.Logger
}

func (w *unmaskingResponseWriter) Write(data []byte) (int, error) {
	// 收集响应数据
	w.body.Write(data)
	return len(data), nil
}

func (w *unmaskingResponseWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

// UnmaskResponse 恢复响应中间件
func (m *PrivacyMaskingMiddleware) UnmaskResponse() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查是否启用了脱敏
		maskingCtx, ok := c.Request.Context().Value(maskingContextKey).(*maskingContext)
		if !ok || !maskingCtx.enabled {
			c.Next()
			return
		}

		// 包装 ResponseWriter
		writer := &unmaskingResponseWriter{
			ResponseWriter: c.Writer,
			service:        m.service,
			ctx:            c.Request.Context(),
			body:           &bytes.Buffer{},
			logger:         m.logger,
		}
		c.Writer = writer

		c.Next()

		// 恢复响应中的占位符
		if writer.body.Len() > 0 {
			unmasked := m.service.UnmaskJSON(c.Request.Context(), writer.body.Bytes())
			
			// 写入恢复后的响应
			writer.ResponseWriter.Write(unmasked)
		}
	}
}

// MaskRequestBody 脱敏请求体（用于 Gateway Service）
func (m *PrivacyMaskingMiddleware) MaskRequestBody(ctx context.Context, body []byte) ([]byte, *MaskingResult, error) {
	config, err := m.service.LoadConfig(ctx)
	if err != nil {
		return body, nil, err
	}

	if !config.Enabled {
		return body, &MaskingResult{Masked: string(body)}, nil
	}

	return m.service.MaskJSON(ctx, body)
}

// UnmaskResponseBody 恢复响应体
func (m *PrivacyMaskingMiddleware) UnmaskResponseBody(ctx context.Context, body []byte) []byte {
	return m.service.UnmaskJSON(ctx, body)
}

// UnmaskStreamChunk 恢复流式响应块
func (m *PrivacyMaskingMiddleware) UnmaskStreamChunk(ctx context.Context, chunk string) string {
	return m.service.Unmask(ctx, chunk)
}

// extractKinds 从检测结果中提取类型列表
func extractKinds(detections []MaskingDetection) []string {
	seen := make(map[string]bool)
	kinds := []string{}
	for _, det := range detections {
		kind := string(det.Kind)
		if !seen[kind] {
			seen[kind] = true
			kinds = append(kinds, kind)
		}
	}
	return kinds
}

// StreamUnmasker 流式响应恢复器
type StreamUnmasker struct {
	service *PrivacyMaskingService
	ctx     context.Context
}

// NewStreamUnmasker 创建流式恢复器
func (m *PrivacyMaskingMiddleware) NewStreamUnmasker(ctx context.Context) *StreamUnmasker {
	return &StreamUnmasker{
		service: m.service,
		ctx:     ctx,
	}
}

// UnmaskChunk 恢复单个 SSE 数据块
func (u *StreamUnmasker) UnmaskChunk(chunk []byte) []byte {
	// SSE 格式: data: {...}\n\n
	chunkStr := string(chunk)
	
	// 处理 SSE 格式的数据
	if strings.HasPrefix(chunkStr, "data: ") {
		lines := strings.Split(chunkStr, "\n")
		var result []string
		
		for _, line := range lines {
			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				
				// 特殊情况：[DONE]
				if data == "[DONE]" {
					result = append(result, line)
					continue
				}
				
				// 解析 JSON
				var jsonData map[string]interface{}
				if err := json.Unmarshal([]byte(data), &jsonData); err != nil {
					// 不是 JSON，直接恢复字符串
					unmasked := u.service.Unmask(u.ctx, data)
					result = append(result, "data: "+unmasked)
				} else {
					// 恢复 JSON 中的占位符
					unmaskedBytes := u.service.UnmaskJSON(u.ctx, []byte(data))
					result = append(result, "data: "+string(unmaskedBytes))
				}
			} else {
				result = append(result, line)
			}
		}
		
		return []byte(strings.Join(result, "\n"))
	}
	
	// 非 SSE 格式，直接恢复
	return []byte(u.service.Unmask(u.ctx, chunkStr))
}

// LogMasking 记录脱敏事件
func (m *PrivacyMaskingMiddleware) LogMasking(ctx context.Context, result *MaskingResult, userID int64, apiKeyID *int64, groupID *int64, requestID, endpoint, model string) error {
	if result == nil || len(result.Detections) == 0 {
		return nil
	}

	// 统计每种类型的数量
	kinds := make(map[string]int)
	actions := make(map[string]PrivacyMaskingAction)
	samples := []PrivacyMaskingValueSample{}

	for _, det := range result.Detections {
		kindStr := string(det.Kind)
		kinds[kindStr]++
		actions[kindStr] = det.Action

		// 只保存前几个示例
		if len(samples) < 10 {
			samples = append(samples, PrivacyMaskingValueSample{
				Kind:   det.Kind,
				Prefix: CreateSamplePrefix(det.Value, 4),
				Length: len(det.Value),
			})
		}
	}

	log := &PrivacyMaskingLog{
		RequestID:      requestID,
		UserID:         userID,
		APIKeyID:       apiKeyID,
		GroupID:        groupID,
		Kinds:          kinds,
		Actions:        actions,
		TotalCount:     len(result.Detections),
		MaskedCount:    result.MaskedCount,
		WarnedCount:    result.WarnedCount,
		BlockedCount:   result.BlockedCount,
		RequestBlocked: result.ShouldBlock,
		ValueSamples:   samples,
		Endpoint:       endpoint,
		Model:          model,
	}

	return m.service.LogMasking(ctx, log)
}
