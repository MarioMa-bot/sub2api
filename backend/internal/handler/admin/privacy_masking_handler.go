package admin

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// PrivacyMaskingHandler 隐私脱敏管理接口：规则配置、脱敏日志、按 API 接口统计
type PrivacyMaskingHandler struct {
	service *service.PrivacyMaskingService
}

// NewPrivacyMaskingHandler 创建处理器实例
func NewPrivacyMaskingHandler(svc *service.PrivacyMaskingService) *PrivacyMaskingHandler {
	return &PrivacyMaskingHandler{service: svc}
}

// GetConfig 获取全局脱敏配置
// GET /api/v1/admin/privacy-masking/config
func (h *PrivacyMaskingHandler) GetConfig(c *gin.Context) {
	cfg, err := h.service.GetConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

// SaveConfig 保存全局脱敏配置，保存后立即生效
// PUT /api/v1/admin/privacy-masking/config
func (h *PrivacyMaskingHandler) SaveConfig(c *gin.Context) {
	var cfg service.PrivacyMaskingConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	if subject, ok := middleware.GetAuthSubjectFromContext(c); ok && subject.UserID > 0 {
		userID := subject.UserID
		cfg.UpdatedByUserID = &userID
	}
	if err := h.service.SaveConfig(c.Request.Context(), &cfg); err != nil {
		if errors.Is(err, service.ErrPrivacyMaskingInvalidConfig) {
			response.BadRequest(c, err.Error())
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	saved, err := h.service.GetConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, saved)
}

// ListLogs 分页查询脱敏日志
// GET /api/v1/admin/privacy-masking/logs
// 过滤参数：endpoint、kind、model、user_id、api_key_id、group_id、blocked、start_time、end_time（RFC3339）
func (h *PrivacyMaskingHandler) ListLogs(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	if pageSize > 200 {
		pageSize = 200
	}
	filter := service.PrivacyMaskingLogFilter{
		Page:     page,
		PageSize: pageSize,
		Endpoint: strings.TrimSpace(c.Query("endpoint")),
		Kind:     strings.TrimSpace(c.Query("kind")),
		Model:    strings.TrimSpace(c.Query("model")),
	}
	for _, p := range []struct {
		name string
		dest **int64
	}{
		{"user_id", &filter.UserID},
		{"api_key_id", &filter.APIKeyID},
		{"group_id", &filter.GroupID},
	} {
		if v := strings.TrimSpace(c.Query(p.name)); v != "" {
			id, err := strconv.ParseInt(v, 10, 64)
			if err != nil || id <= 0 {
				response.BadRequest(c, "Invalid "+p.name)
				return
			}
			*p.dest = &id
		}
	}
	if v := strings.TrimSpace(c.Query("blocked")); v != "" {
		blocked, err := strconv.ParseBool(v)
		if err != nil {
			response.BadRequest(c, "Invalid blocked, expect true or false")
			return
		}
		filter.Blocked = &blocked
	}
	var ok bool
	if filter.StartTime, filter.EndTime, ok = parsePrivacyMaskingTimeRange(c); !ok {
		return
	}

	logs, total, err := h.service.ListLogs(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, logs, total, page, pageSize)
}

// ListEndpoints 按 API 接口聚合脱敏事件
// GET /api/v1/admin/privacy-masking/logs/endpoints
func (h *PrivacyMaskingHandler) ListEndpoints(c *gin.Context) {
	start, end, ok := parsePrivacyMaskingTimeRange(c)
	if !ok {
		return
	}
	items, err := h.service.ListEndpointSummary(c.Request.Context(), start, end)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

// GetStats 统计概览，since 默认为 24 小时前
// GET /api/v1/admin/privacy-masking/stats
func (h *PrivacyMaskingHandler) GetStats(c *gin.Context) {
	since := time.Now().Add(-24 * time.Hour)
	if v := strings.TrimSpace(c.Query("since")); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			response.BadRequest(c, "Invalid since, expect RFC3339")
			return
		}
		since = t
	}
	stats, err := h.service.GetStats(c.Request.Context(), since)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, stats)
}

func parsePrivacyMaskingTimeRange(c *gin.Context) (*time.Time, *time.Time, bool) {
	var start, end *time.Time
	for _, p := range []struct {
		name string
		dest **time.Time
	}{
		{"start_time", &start},
		{"end_time", &end},
	} {
		if v := strings.TrimSpace(c.Query(p.name)); v != "" {
			t, err := time.Parse(time.RFC3339, v)
			if err != nil {
				response.Error(c, http.StatusBadRequest, "Invalid "+p.name+", expect RFC3339")
				return nil, nil, false
			}
			*p.dest = &t
		}
	}
	return start, end, true
}
