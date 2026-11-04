package repository

import (
	"context"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"gorm.io/gorm"
)

// PrivacyMaskingRepository 隐私脱敏配置仓库
type PrivacyMaskingRepository struct {
	db *gorm.DB
}

// NewPrivacyMaskingRepository 创建仓库实例
func NewPrivacyMaskingRepository(db *gorm.DB) *PrivacyMaskingRepository {
	return &PrivacyMaskingRepository{db: db}
}

// GetGlobalConfig 获取全局配置
func (r *PrivacyMaskingRepository) GetGlobalConfig(ctx context.Context) (*service.PrivacyMaskingConfig, error) {
	var config service.PrivacyMaskingConfig
	err := r.db.WithContext(ctx).
		Where("group_id IS NULL").
		First(&config).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// 返回默认配置
			return &service.PrivacyMaskingConfig{
				Enabled:      false,
				MaskSecrets:  true,
				MaskPersonal: false,
				MaskedKinds:  make(map[string]bool),
				CustomRules:  []service.PrivacyMaskingRule{},
				AllowList:    []string{},
				KindActions:  make(map[string]service.PrivacyMaskingAction),
			}, nil
		}
		return nil, fmt.Errorf("failed to get global config: %w", err)
	}
	return &config, nil
}

// GetGroupConfig 获取分组配置（如果不存在则返回全局配置）
func (r *PrivacyMaskingRepository) GetGroupConfig(ctx context.Context, groupID int64) (*service.PrivacyMaskingConfig, error) {
	var config service.PrivacyMaskingConfig
	err := r.db.WithContext(ctx).
		Where("group_id = ?", groupID).
		First(&config).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// 返回全局配置
			return r.GetGlobalConfig(ctx)
		}
		return nil, fmt.Errorf("failed to get group config: %w", err)
	}
	return &config, nil
}

// UpdateGlobalConfig 更新全局配置
func (r *PrivacyMaskingRepository) UpdateGlobalConfig(ctx context.Context, config *service.PrivacyMaskingConfig) error {
	config.GroupID = nil
	err := r.db.WithContext(ctx).
		Where("group_id IS NULL").
		Save(config).Error
	if err != nil {
		return fmt.Errorf("failed to update global config: %w", err)
	}
	return nil
}

// UpdateGroupConfig 更新分组配置
func (r *PrivacyMaskingRepository) UpdateGroupConfig(ctx context.Context, groupID int64, config *service.PrivacyMaskingConfig) error {
	config.GroupID = &groupID
	err := r.db.WithContext(ctx).Save(config).Error
	if err != nil {
		return fmt.Errorf("failed to update group config: %w", err)
	}
	return nil
}

// CreateLog 创建脱敏日志
func (r *PrivacyMaskingRepository) CreateLog(ctx context.Context, log *service.PrivacyMaskingLog) error {
	err := r.db.WithContext(ctx).Create(log).Error
	if err != nil {
		return fmt.Errorf("failed to create masking log: %w", err)
	}
	return nil
}

// ListLogs 列出脱敏日志
func (r *PrivacyMaskingRepository) ListLogs(ctx context.Context, userID *int64, groupID *int64, blocked *bool, limit int, offset int) ([]service.PrivacyMaskingLog, int64, error) {
	query := r.db.WithContext(ctx).Model(&service.PrivacyMaskingLog{})

	if userID != nil {
		query = query.Where("user_id = ?", *userID)
	}
	if groupID != nil {
		query = query.Where("group_id = ?", *groupID)
	}
	if blocked != nil {
		query = query.Where("request_blocked = ?", *blocked)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count logs: %w", err)
	}

	var logs []service.PrivacyMaskingLog
	err := query.
		Order("detected_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&logs).Error
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list logs: %w", err)
	}

	return logs, total, nil
}
