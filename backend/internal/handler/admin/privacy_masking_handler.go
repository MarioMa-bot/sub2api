package admin

import (
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// PrivacyMaskingHandler 隐私脱敏管理处理器
type PrivacyMaskingHandler struct {
	repo   *repository.PrivacyMaskingRepository
	logger *zap.Logger
}

// NewPrivacyMaskingHandler 创建处理器实例
func NewPrivacyMaskingHandler(repo *repository.PrivacyMaskingRepository, logger *zap.Logger) *PrivacyMaskingHandler {
	return &PrivacyMaskingHandler{
		repo:   repo,
		logger: logger,
	}
}

// GetGlobalConfig 获取全局配置
// GET /admin/privacy-masking/config
func (h *PrivacyMaskingHandler) GetGlobalConfig(c *gin.Context) {
	ctx := c.Request.Context()

	config, err := h.repo.GetGlobalConfig(ctx)
	if err != nil {
		h.logger.Error("Failed to get global config", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get configuration"})
		return
	}

	c.JSON(http.StatusOK, config)
}

// SaveGlobalConfig 更新全局配置
// PUT /admin/privacy-masking/config
func (h *PrivacyMaskingHandler) SaveGlobalConfig(c *gin.Context) {
	ctx := c.Request.Context()

	var config service.PrivacyMaskingConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// TODO: 从认证上下文获取用户 ID
	// config.UpdatedByUserID = &userID

	if err := h.repo.SaveGlobalConfig(ctx, &config); err != nil {
		h.logger.Error("Failed to update global config", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update configuration"})
		return
	}

	c.JSON(http.StatusOK, config)
}

// GetGroupConfig 获取分组配置
// GET /admin/privacy-masking/groups/:groupId/config
func (h *PrivacyMaskingHandler) GetGroupConfig(c *gin.Context) {
	ctx := c.Request.Context()

	groupID, err := strconv.ParseInt(c.Param("groupId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group ID"})
		return
	}

	config, err := h.repo.GetGroupConfig(ctx, groupID)
	if err != nil {
		h.logger.Error("Failed to get group config", zap.Int64("group_id", groupID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get configuration"})
		return
	}

	c.JSON(http.StatusOK, config)
}

// SaveGroupConfig 更新分组配置
// PUT /admin/privacy-masking/groups/:groupId/config
func (h *PrivacyMaskingHandler) SaveGroupConfig(c *gin.Context) {
	ctx := c.Request.Context()

	groupID, err := strconv.ParseInt(c.Param("groupId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group ID"})
		return
	}

	var config service.PrivacyMaskingConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if err := h.repo.SaveGroupConfig(ctx, groupID, &config); err != nil {
		h.logger.Error("Failed to update group config", zap.Int64("group_id", groupID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update configuration"})
		return
	}

	c.JSON(http.StatusOK, config)
}

// ListLogs 列出脱敏日志
// GET /admin/privacy-masking/logs
func (h *PrivacyMaskingHandler) ListLogs(c *gin.Context) {
	ctx := c.Request.Context()

	// 解析查询参数（简化版 - 暂不支持过滤）
	limit := 50
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 200 {
			limit = l
		}
	}

	offset := 0
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	logs, total, err := h.repo.ListLogs(ctx, limit, offset)
	if err != nil {
		h.logger.Error("Failed to list logs", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list logs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"logs":   logs,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetStats 获取脱敏统计
// GET /admin/privacy-masking/stats
func (h *PrivacyMaskingHandler) GetStats(c *gin.Context) {
	_ctx := c.Request.Context()
	_ = _ctx // unused for now

	// TODO: 实现统计查询
	// 例如：最近检测到的类型分布、阻止的请求数量等

	c.JSON(http.StatusOK, gin.H{
		"total_detections": 0,
		"blocked_requests": 0,
		"top_kinds":        []string{},
	})
}
