package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

// privacyMaskingLogTimeout 异步写日志的超时时间
const privacyMaskingLogTimeout = 5 * time.Second

// privacyMaskingMaxPlaceholderLen 流式输出时最多暂存的未闭合占位符长度
const privacyMaskingMaxPlaceholderLen = 64

const privacyMaskingFieldMaxLen = 255

// partialPlaceholderPattern 判断末尾片段是否可能是被拆开的占位符
var partialPlaceholderPattern = regexp.MustCompile(`^\{\{[A-Za-z0-9_]*\}?$`)

// PrivacyMaskingIdentity 请求的调用方身份（用于日志）
type PrivacyMaskingIdentity struct {
	UserID   int64
	APIKeyID *int64
	GroupID  *int64
}

// PrivacyMaskingIdentityResolver 从请求上下文解析调用方身份。
// service 包不能依赖 server/middleware，因此由路由层注入。
type PrivacyMaskingIdentityResolver func(c *gin.Context) PrivacyMaskingIdentity

// PrivacyMaskingMiddleware 隐私脱敏中间件
type PrivacyMaskingMiddleware struct {
	service *PrivacyMaskingService
	// logWriter 写日志的执行方式；默认异步，测试可替换为同步
	logWriter func(fn func())
}

// NewPrivacyMaskingMiddleware 创建中间件实例
func NewPrivacyMaskingMiddleware(service *PrivacyMaskingService) *PrivacyMaskingMiddleware {
	return &PrivacyMaskingMiddleware{
		service:   service,
		logWriter: func(fn func()) { go fn() },
	}
}

// Handler 网关脱敏中间件：
//  1. 请求转发前把 JSON 请求体中的敏感值替换为 {{KIND_hash}} 占位符；
//  2. 命中任何规则（mask/warn/block）都写一条仅含元数据的日志；
//  3. 命中 block 规则或脱敏失败时直接 403 阻断，绝不放行原文；
//  4. 响应（含 SSE 流）边写边把本次请求产生的占位符恢复为原始值。
//
// 只检查 JSON 请求体；multipart、二进制等非 JSON 请求原样放行。
func (m *PrivacyMaskingMiddleware) Handler(resolve PrivacyMaskingIdentityResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		if m == nil || m.service == nil || c.Request.Body == nil {
			c.Next()
			return
		}
		switch c.Request.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch:
		default:
			c.Next()
			return
		}

		cfg, err := m.service.LoadConfig(c.Request.Context())
		if err != nil {
			logger.L().Error("privacy masking: load config failed", zap.Error(err))
			abortPrivacyMasking(c, http.StatusServiceUnavailable, "privacy_masking_unavailable", "Privacy masking is unavailable, request rejected for safety")
			return
		}
		if !cfg.Enabled {
			c.Next()
			return
		}

		body, err := io.ReadAll(c.Request.Body)
		_ = c.Request.Body.Close()
		if err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				abortPrivacyMasking(c, http.StatusRequestEntityTooLarge, "request_too_large", "Request body too large")
				return
			}
			abortPrivacyMasking(c, http.StatusBadRequest, "privacy_masking_error", "Failed to read request body")
			return
		}
		setRequestBody(c, body)
		if len(body) == 0 || !json.Valid(body) {
			c.Next()
			return
		}

		masked, result, maskErr := m.service.MaskJSON(c.Request.Context(), body)
		if result != nil && len(result.Detections) > 0 {
			blocked := maskErr != nil || result.ShouldBlock
			m.recordLog(c, resolve, body, result, blocked)
		}
		if maskErr != nil {
			logger.L().Error("privacy masking: mask request failed", zap.Error(maskErr), zap.String("path", c.Request.URL.Path))
			abortPrivacyMasking(c, http.StatusForbidden, "privacy_masking_error", "Request masking failed for security reasons")
			return
		}
		if result.ShouldBlock {
			logger.L().Warn("privacy masking: request blocked",
				zap.String("path", c.Request.URL.Path),
				zap.Int("blocked_count", result.BlockedCount),
				zap.Strings("kinds", extractKinds(result.Detections)))
			abortPrivacyMasking(c, http.StatusForbidden, "privacy_masking_blocked", "Request contains blocked sensitive information")
			return
		}
		if len(result.Placeholders) == 0 {
			c.Next()
			return
		}

		setRequestBody(c, masked)
		writer := &unmaskingResponseWriter{ResponseWriter: c.Writer, placeholders: result.Placeholders}
		c.Writer = writer
		defer func() {
			writer.finish()
			c.Writer = writer.ResponseWriter
		}()
		c.Next()
	}
}

func setRequestBody(c *gin.Context, body []byte) {
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	c.Request.ContentLength = int64(len(body))
}

func abortPrivacyMasking(c *gin.Context, status int, errType, message string) {
	c.AbortWithStatusJSON(status, gin.H{
		"error": gin.H{"type": errType, "message": message},
	})
}

// recordLog 写入脱敏日志（仅元数据，不含原始值）
func (m *PrivacyMaskingMiddleware) recordLog(c *gin.Context, resolve PrivacyMaskingIdentityResolver, body []byte, result *MaskingResult, blocked bool) {
	entry := BuildPrivacyMaskingLog(result)
	if entry == nil {
		return
	}
	entry.RequestBlocked = blocked
	if resolve != nil {
		identity := resolve(c)
		entry.UserID, entry.APIKeyID, entry.GroupID = identity.UserID, identity.APIKeyID, identity.GroupID
	}
	if requestID, ok := c.Request.Context().Value(ctxkey.ClientRequestID).(string); ok {
		entry.RequestID = truncatePrivacyField(requestID)
	}
	entry.Endpoint, entry.Model = privacyMaskingEndpointAndModel(c, body)

	m.logWriter(func() {
		ctx, cancel := context.WithTimeout(context.Background(), privacyMaskingLogTimeout)
		defer cancel()
		if err := m.service.LogMasking(ctx, entry); err != nil {
			logger.L().Error("privacy masking: write log failed", zap.Error(err), zap.String("endpoint", entry.Endpoint))
		}
	})
}

// privacyMaskingEndpointAndModel 生成用于分组的 API 接口名及模型名。
// 带路径参数的路由使用路由模板（避免 ID 打散分组）；Gemini 的 /models/{model}:{action} 归一为 {model} 占位。
func privacyMaskingEndpointAndModel(c *gin.Context, body []byte) (string, string) {
	model := gjson.GetBytes(body, "model").String()
	endpoint := c.Request.URL.Path
	fullPath := c.FullPath()
	if modelAction := strings.TrimPrefix(c.Param("modelAction"), "/"); modelAction != "" {
		name, action, _ := strings.Cut(modelAction, ":")
		if model == "" {
			model = name
		}
		endpoint = strings.TrimSuffix(fullPath, "*modelAction") + "{model}"
		if action != "" {
			endpoint += ":" + action
		}
	} else if strings.Contains(fullPath, "/:") {
		endpoint = fullPath
	}
	return truncatePrivacyField(endpoint), truncatePrivacyField(model)
}

func truncatePrivacyField(s string) string {
	if len(s) > privacyMaskingFieldMaxLen {
		return s[:privacyMaskingFieldMaxLen]
	}
	return s
}

// unmaskingResponseWriter 边写边恢复占位符，不缓冲整个响应，SSE 可正常逐块下发。
// 只有当本次写入的末尾可能是被拆开的占位符时才暂存这一小段（最多 64 字节）。
type unmaskingResponseWriter struct {
	gin.ResponseWriter
	placeholders map[string]string
	pending      []byte
}

// 恢复后长度会变化，必须去掉上游带来的 Content-Length
func (w *unmaskingResponseWriter) WriteHeader(code int) {
	w.Header().Del("Content-Length")
	w.ResponseWriter.WriteHeader(code)
}

func (w *unmaskingResponseWriter) WriteHeaderNow() {
	w.Header().Del("Content-Length")
	w.ResponseWriter.WriteHeaderNow()
}

func (w *unmaskingResponseWriter) Write(data []byte) (int, error) {
	w.Header().Del("Content-Length")
	buf := append(w.pending, data...)
	emit, keep := splitPartialPlaceholder(buf)
	w.pending = append([]byte(nil), keep...)
	if len(emit) > 0 {
		if _, err := w.ResponseWriter.Write([]byte(UnmaskWith(w.placeholders, string(emit)))); err != nil {
			return 0, err
		}
	}
	return len(data), nil
}

func (w *unmaskingResponseWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

// finish 写出暂存的尾部
func (w *unmaskingResponseWriter) finish() {
	if len(w.pending) == 0 {
		return
	}
	tail := w.pending
	w.pending = nil
	_, _ = w.ResponseWriter.Write([]byte(UnmaskWith(w.placeholders, string(tail))))
}

// splitPartialPlaceholder 把缓冲区拆成可立即输出的部分和可能是未完整占位符的尾部
func splitPartialPlaceholder(buf []byte) (emit, keep []byte) {
	if len(buf) == 0 {
		return buf, nil
	}
	if idx := bytes.LastIndex(buf, []byte("{{")); idx >= 0 && len(buf)-idx <= privacyMaskingMaxPlaceholderLen {
		if tail := buf[idx:]; !bytes.Contains(tail, []byte("}}")) && partialPlaceholderPattern.Match(tail) {
			return buf[:idx], tail
		}
	}
	if buf[len(buf)-1] == '{' {
		return buf[:len(buf)-1], buf[len(buf)-1:]
	}
	return buf, nil
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
