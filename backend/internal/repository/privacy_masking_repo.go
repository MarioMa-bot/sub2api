package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// privacyMaskingRepository 隐私脱敏配置与日志仓储（raw SQL）
type privacyMaskingRepository struct {
	db *sql.DB
}

// NewPrivacyMaskingRepository 创建仓储实例
func NewPrivacyMaskingRepository(db *sql.DB) service.PrivacyMaskingStore {
	return &privacyMaskingRepository{db: db}
}

const privacyMaskingConfigColumns = `id, enabled, mask_secrets, mask_personal, masked_kinds, custom_rules,
allow_list, kind_actions, group_id, created_at, updated_at, updated_by_user_id`

// GetGlobalConfig 获取全局配置（group_id IS NULL 的那一行）
func (r *privacyMaskingRepository) GetGlobalConfig(ctx context.Context) (*service.PrivacyMaskingConfig, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+privacyMaskingConfigColumns+`
FROM privacy_masking_config WHERE group_id IS NULL ORDER BY id LIMIT 1`)
	cfg, err := scanPrivacyMaskingConfig(row)
	if errors.Is(err, sql.ErrNoRows) {
		// 迁移会插入默认行；被手工删除时按默认（关闭）处理
		return &service.PrivacyMaskingConfig{
			MaskSecrets: true,
			MaskedKinds: map[string]bool{},
			CustomRules: []service.PrivacyMaskingRule{},
			AllowList:   []string{},
			KindActions: map[string]service.PrivacyMaskingAction{},
		}, nil
	}
	return cfg, err
}

// SaveGlobalConfig 保存全局配置
func (r *privacyMaskingRepository) SaveGlobalConfig(ctx context.Context, cfg *service.PrivacyMaskingConfig) error {
	maskedKinds, customRules, allowList, kindActions, err := marshalPrivacyMaskingConfigJSON(cfg)
	if err != nil {
		return err
	}
	args := []any{cfg.Enabled, cfg.MaskSecrets, cfg.MaskPersonal, maskedKinds, customRules, allowList, kindActions, nullInt64Ptr(cfg.UpdatedByUserID)}
	res, err := r.db.ExecContext(ctx, `UPDATE privacy_masking_config
SET enabled = $1, mask_secrets = $2, mask_personal = $3, masked_kinds = $4::jsonb, custom_rules = $5::jsonb,
    allow_list = $6::jsonb, kind_actions = $7::jsonb, updated_by_user_id = $8
WHERE group_id IS NULL`, args...)
	if err != nil {
		return fmt.Errorf("update privacy masking config: %w", err)
	}
	if n, _ := res.RowsAffected(); n > 0 {
		return nil
	}
	if _, err := r.db.ExecContext(ctx, `INSERT INTO privacy_masking_config
(enabled, mask_secrets, mask_personal, masked_kinds, custom_rules, allow_list, kind_actions, updated_by_user_id)
VALUES ($1, $2, $3, $4::jsonb, $5::jsonb, $6::jsonb, $7::jsonb, $8)`, args...); err != nil {
		return fmt.Errorf("insert privacy masking config: %w", err)
	}
	return nil
}

// InsertLog 写入一条脱敏日志（仅元数据）
func (r *privacyMaskingRepository) InsertLog(ctx context.Context, log *service.PrivacyMaskingLog) error {
	kinds, err := json.Marshal(nonNilMap(log.Kinds))
	if err != nil {
		return err
	}
	actions, err := json.Marshal(nonNilActionMap(log.Actions))
	if err != nil {
		return err
	}
	var samples any
	if len(log.ValueSamples) > 0 {
		encoded, err := json.Marshal(log.ValueSamples)
		if err != nil {
			return err
		}
		samples = string(encoded)
	}
	detectedAt := log.DetectedAt
	if detectedAt.IsZero() {
		detectedAt = time.Now()
	}
	err = r.db.QueryRowContext(ctx, `INSERT INTO privacy_masking_log
(request_id, user_id, api_key_id, group_id, detected_at, kinds, actions, total_count, masked_count,
 warned_count, blocked_count, request_blocked, value_samples, endpoint, model)
VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7::jsonb, $8, $9, $10, $11, $12, $13::jsonb, $14, $15)
RETURNING id, created_at`,
		privacyNullString(truncateString(log.RequestID, 255)),
		log.UserID,
		nullInt64Ptr(log.APIKeyID),
		nullInt64Ptr(log.GroupID),
		detectedAt.UTC(),
		string(kinds),
		string(actions),
		log.TotalCount,
		log.MaskedCount,
		log.WarnedCount,
		log.BlockedCount,
		log.RequestBlocked,
		samples,
		privacyNullString(truncateString(log.Endpoint, 255)),
		privacyNullString(truncateString(log.Model, 255)),
	).Scan(&log.ID, &log.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert privacy masking log: %w", err)
	}
	return nil
}

// buildPrivacyMaskingLogWhere 生成日志查询条件
func buildPrivacyMaskingLogWhere(filter service.PrivacyMaskingLogFilter) (string, []any) {
	clauses := []string{"1=1"}
	args := []any{}
	add := func(clause string, value any) {
		args = append(args, value)
		clauses = append(clauses, fmt.Sprintf(clause, len(args)))
	}
	if v := strings.TrimSpace(filter.Endpoint); v != "" {
		add("endpoint = $%d", v)
	}
	if v := strings.TrimSpace(filter.Kind); v != "" {
		add("kinds ? $%d", strings.ToUpper(v))
	}
	if v := strings.TrimSpace(filter.Model); v != "" {
		add("model ILIKE $%d", "%"+escapeLike(v)+"%")
	}
	if filter.UserID != nil {
		add("user_id = $%d", *filter.UserID)
	}
	if filter.APIKeyID != nil {
		add("api_key_id = $%d", *filter.APIKeyID)
	}
	if filter.GroupID != nil {
		add("group_id = $%d", *filter.GroupID)
	}
	if filter.Blocked != nil {
		add("request_blocked = $%d", *filter.Blocked)
	}
	if filter.StartTime != nil {
		add("detected_at >= $%d", filter.StartTime.UTC())
	}
	if filter.EndTime != nil {
		add("detected_at <= $%d", filter.EndTime.UTC())
	}
	return strings.Join(clauses, " AND "), args
}

// ListLogs 分页查询脱敏日志（按检测时间倒序）
func (r *privacyMaskingRepository) ListLogs(ctx context.Context, filter service.PrivacyMaskingLogFilter) ([]service.PrivacyMaskingLog, int64, error) {
	page, pageSize := filter.Page, filter.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}
	where, args := buildPrivacyMaskingLogWhere(filter)

	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM privacy_masking_log WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count privacy masking logs: %w", err)
	}
	if total == 0 {
		return []service.PrivacyMaskingLog{}, 0, nil
	}

	query := fmt.Sprintf(`SELECT id, COALESCE(request_id, ''), user_id, api_key_id, group_id, detected_at, kinds, actions,
total_count, masked_count, warned_count, blocked_count, request_blocked, value_samples,
COALESCE(endpoint, ''), COALESCE(model, ''), created_at
FROM privacy_masking_log WHERE %s ORDER BY detected_at DESC, id DESC LIMIT $%d OFFSET $%d`, where, len(args)+1, len(args)+2)
	rows, err := r.db.QueryContext(ctx, query, append(args, pageSize, (page-1)*pageSize)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list privacy masking logs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	logs := make([]service.PrivacyMaskingLog, 0, pageSize)
	for rows.Next() {
		var (
			item              service.PrivacyMaskingLog
			apiKeyID, groupID sql.NullInt64
			kinds, actions    []byte
			samples           []byte
		)
		if err := rows.Scan(&item.ID, &item.RequestID, &item.UserID, &apiKeyID, &groupID, &item.DetectedAt,
			&kinds, &actions, &item.TotalCount, &item.MaskedCount, &item.WarnedCount, &item.BlockedCount,
			&item.RequestBlocked, &samples, &item.Endpoint, &item.Model, &item.CreatedAt); err != nil {
			return nil, 0, err
		}
		if apiKeyID.Valid {
			item.APIKeyID = &apiKeyID.Int64
		}
		if groupID.Valid {
			item.GroupID = &groupID.Int64
		}
		item.Kinds = map[string]int{}
		item.Actions = map[string]service.PrivacyMaskingAction{}
		_ = json.Unmarshal(kinds, &item.Kinds)
		_ = json.Unmarshal(actions, &item.Actions)
		if len(samples) > 0 {
			_ = json.Unmarshal(samples, &item.ValueSamples)
		}
		logs = append(logs, item)
	}
	return logs, total, rows.Err()
}

// ListEndpointSummary 按 API 接口聚合
func (r *privacyMaskingRepository) ListEndpointSummary(ctx context.Context, start, end *time.Time) ([]service.PrivacyMaskingEndpointSummary, error) {
	where, args := buildPrivacyMaskingLogWhere(service.PrivacyMaskingLogFilter{StartTime: start, EndTime: end})
	rows, err := r.db.QueryContext(ctx, `SELECT COALESCE(endpoint, ''), COUNT(*),
COUNT(*) FILTER (WHERE request_blocked), COALESCE(SUM(total_count), 0), COALESCE(SUM(masked_count), 0),
COALESCE(SUM(warned_count), 0), MAX(detected_at)
FROM privacy_masking_log WHERE `+where+`
GROUP BY COALESCE(endpoint, '') ORDER BY COUNT(*) DESC, MAX(detected_at) DESC LIMIT 200`, args...)
	if err != nil {
		return nil, fmt.Errorf("summarize privacy masking logs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := []service.PrivacyMaskingEndpointSummary{}
	for rows.Next() {
		var item service.PrivacyMaskingEndpointSummary
		if err := rows.Scan(&item.Endpoint, &item.EventCount, &item.BlockedCount, &item.DetectedCount,
			&item.MaskedCount, &item.WarnedCount, &item.LastSeenAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// GetStats 统计概览
func (r *privacyMaskingRepository) GetStats(ctx context.Context, since time.Time) (*service.PrivacyMaskingStats, error) {
	stats := &service.PrivacyMaskingStats{Since: since, ByKind: map[string]int64{}}
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*), COUNT(*) FILTER (WHERE request_blocked),
COALESCE(SUM(total_count), 0), COALESCE(SUM(masked_count), 0), COALESCE(SUM(warned_count), 0)
FROM privacy_masking_log WHERE detected_at >= $1`, since.UTC()).Scan(
		&stats.EventCount, &stats.BlockedCount, &stats.DetectedCount, &stats.MaskedCount, &stats.WarnedCount); err != nil {
		return nil, fmt.Errorf("privacy masking stats: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, `SELECT k.key, COALESCE(SUM((k.value)::bigint), 0)
FROM privacy_masking_log l, jsonb_each_text(l.kinds) AS k
WHERE l.detected_at >= $1 GROUP BY k.key`, since.UTC())
	if err != nil {
		return nil, fmt.Errorf("privacy masking stats by kind: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var kind string
		var count int64
		if err := rows.Scan(&kind, &count); err != nil {
			return nil, err
		}
		stats.ByKind[kind] = count
	}
	return stats, rows.Err()
}

type privacyMaskingRowScanner interface {
	Scan(dest ...any) error
}

func scanPrivacyMaskingConfig(row privacyMaskingRowScanner) (*service.PrivacyMaskingConfig, error) {
	var (
		cfg                                              service.PrivacyMaskingConfig
		maskedKinds, customRules, allowList, kindActions []byte
		groupID, updatedBy                               sql.NullInt64
	)
	if err := row.Scan(&cfg.ID, &cfg.Enabled, &cfg.MaskSecrets, &cfg.MaskPersonal, &maskedKinds, &customRules,
		&allowList, &kindActions, &groupID, &cfg.CreatedAt, &cfg.UpdatedAt, &updatedBy); err != nil {
		return nil, err
	}
	cfg.MaskedKinds = map[string]bool{}
	cfg.CustomRules = []service.PrivacyMaskingRule{}
	cfg.AllowList = []string{}
	cfg.KindActions = map[string]service.PrivacyMaskingAction{}
	for _, part := range []struct {
		raw  []byte
		dest any
		name string
	}{
		{maskedKinds, &cfg.MaskedKinds, "masked_kinds"},
		{customRules, &cfg.CustomRules, "custom_rules"},
		{allowList, &cfg.AllowList, "allow_list"},
		{kindActions, &cfg.KindActions, "kind_actions"},
	} {
		if len(part.raw) == 0 {
			continue
		}
		if err := json.Unmarshal(part.raw, part.dest); err != nil {
			return nil, fmt.Errorf("decode privacy masking %s: %w", part.name, err)
		}
	}
	if groupID.Valid {
		cfg.GroupID = &groupID.Int64
	}
	if updatedBy.Valid {
		cfg.UpdatedByUserID = &updatedBy.Int64
	}
	return &cfg, nil
}

func marshalPrivacyMaskingConfigJSON(cfg *service.PrivacyMaskingConfig) (string, string, string, string, error) {
	parts := []any{cfg.MaskedKinds, cfg.CustomRules, cfg.AllowList, cfg.KindActions}
	defaults := []string{"{}", "[]", "[]", "{}"}
	out := make([]string, len(parts))
	for i, part := range parts {
		encoded, err := json.Marshal(part)
		if err != nil {
			return "", "", "", "", err
		}
		if string(encoded) == "null" {
			encoded = []byte(defaults[i])
		}
		out[i] = string(encoded)
	}
	return out[0], out[1], out[2], out[3], nil
}

func nonNilMap(m map[string]int) map[string]int {
	if m == nil {
		return map[string]int{}
	}
	return m
}

func nonNilActionMap(m map[string]service.PrivacyMaskingAction) map[string]service.PrivacyMaskingAction {
	if m == nil {
		return map[string]service.PrivacyMaskingAction{}
	}
	return m
}

func privacyNullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
