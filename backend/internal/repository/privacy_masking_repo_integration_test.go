//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestPrivacyMaskingRepository_ConfigAndLogs(t *testing.T) {
	ctx := context.Background()
	_, err := integrationDB.ExecContext(ctx, `TRUNCATE privacy_masking_log`)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `UPDATE privacy_masking_config SET enabled = false, kind_actions = '{}', allow_list = '[]', custom_rules = '[]',
    smart_masking_enabled = false, smart_sidecar_url = '', smart_timeout_ms = 80, smart_min_score = 0.6, smart_entities = '["PERSON","LOCATION","ORG"]'
WHERE group_id IS NULL`)
	require.NoError(t, err)
	repo := NewPrivacyMaskingRepository(integrationDB)

	// 迁移插入的默认全局配置：关闭
	cfg, err := repo.GetGlobalConfig(ctx)
	require.NoError(t, err)
	require.False(t, cfg.Enabled)
	require.False(t, cfg.SmartMaskingEnabled)
	require.Equal(t, 80, cfg.SmartTimeoutMS)
	require.Equal(t, []string{"PERSON", "LOCATION", "ORG"}, cfg.SmartEntities)

	adminID := int64(1)
	cfg.Enabled = true
	cfg.KindActions = map[string]service.PrivacyMaskingAction{"API_KEY": service.PrivacyMaskingActionBlock}
	cfg.AllowList = []string{"sk-allowed"}
	cfg.CustomRules = []service.PrivacyMaskingRule{{Pattern: `secret-\w+`, Kind: "INTERNAL", Action: service.PrivacyMaskingActionMask}}
	cfg.UpdatedByUserID = &adminID
	cfg.SmartMaskingEnabled = true
	cfg.SmartSidecarURL = "http://privacy-ner:5002"
	cfg.SmartTimeoutMS = 120
	cfg.SmartMinScore = 0.75
	cfg.SmartEntities = []string{"PERSON"}
	require.NoError(t, repo.SaveGlobalConfig(ctx, cfg))

	saved, err := repo.GetGlobalConfig(ctx)
	require.NoError(t, err)
	require.True(t, saved.Enabled)
	require.Equal(t, service.PrivacyMaskingActionBlock, saved.KindActions["API_KEY"])
	require.Equal(t, []string{"sk-allowed"}, saved.AllowList)
	require.Len(t, saved.CustomRules, 1)
	require.Equal(t, adminID, *saved.UpdatedByUserID)
	require.True(t, saved.SmartMaskingEnabled)
	require.Equal(t, "http://privacy-ner:5002", saved.SmartSidecarURL)
	require.Equal(t, 120, saved.SmartTimeoutMS)
	require.InDelta(t, 0.75, saved.SmartMinScore, 1e-9)
	require.Equal(t, []string{"PERSON"}, saved.SmartEntities)

	keyID, groupID := int64(42), int64(9)
	now := time.Now().UTC()
	entries := []*service.PrivacyMaskingLog{
		{UserID: 7, APIKeyID: &keyID, GroupID: &groupID, DetectedAt: now.Add(-2 * time.Hour), Endpoint: "/v1/messages", Model: "claude-sonnet-4",
			Kinds: map[string]int{"API_KEY": 2}, Actions: map[string]service.PrivacyMaskingAction{"API_KEY": "mask"}, TotalCount: 2, MaskedCount: 2,
			ValueSamples: []service.PrivacyMaskingValueSample{{Kind: service.KindAPIKey, Prefix: "sk-a...", Length: 48}}, RequestID: "req-1"},
		{UserID: 7, APIKeyID: &keyID, DetectedAt: now.Add(-time.Hour), Endpoint: "/v1/messages", Model: "claude-opus-4",
			Kinds: map[string]int{"EMAIL": 1}, Actions: map[string]service.PrivacyMaskingAction{"EMAIL": "block"}, TotalCount: 1, BlockedCount: 1, RequestBlocked: true},
		{UserID: 8, DetectedAt: now, Endpoint: "/v1/chat/completions", Model: "gpt-5",
			Kinds: map[string]int{"API_KEY": 1, "EMAIL": 1}, Actions: map[string]service.PrivacyMaskingAction{"API_KEY": "mask", "EMAIL": "warn"}, TotalCount: 2, MaskedCount: 1, WarnedCount: 1},
	}
	for _, e := range entries {
		require.NoError(t, repo.InsertLog(ctx, e))
		require.NotZero(t, e.ID)
	}

	list := func(f service.PrivacyMaskingLogFilter) ([]service.PrivacyMaskingLog, int64) {
		t.Helper()
		items, total, err := repo.ListLogs(ctx, f)
		require.NoError(t, err)
		return items, total
	}

	items, total := list(service.PrivacyMaskingLogFilter{Page: 1, PageSize: 2})
	require.Equal(t, int64(3), total)
	require.Len(t, items, 2)
	require.Equal(t, "/v1/chat/completions", items[0].Endpoint, "newest first")

	items, total = list(service.PrivacyMaskingLogFilter{Endpoint: "/v1/messages"})
	require.Equal(t, int64(2), total)
	require.Equal(t, "req-1", items[1].RequestID)
	require.Equal(t, 2, items[1].Kinds["API_KEY"])
	require.Equal(t, "sk-a...", items[1].ValueSamples[0].Prefix)
	require.Equal(t, groupID, *items[1].GroupID)

	_, total = list(service.PrivacyMaskingLogFilter{Kind: "email"})
	require.Equal(t, int64(2), total)

	blocked := true
	items, total = list(service.PrivacyMaskingLogFilter{Blocked: &blocked})
	require.Equal(t, int64(1), total)
	require.True(t, items[0].RequestBlocked)

	_, total = list(service.PrivacyMaskingLogFilter{Model: "OPUS"})
	require.Equal(t, int64(1), total)

	userID := int64(8)
	_, total = list(service.PrivacyMaskingLogFilter{UserID: &userID})
	require.Equal(t, int64(1), total)

	start := now.Add(-90 * time.Minute)
	_, total = list(service.PrivacyMaskingLogFilter{StartTime: &start})
	require.Equal(t, int64(2), total)

	summary, err := repo.ListEndpointSummary(ctx, nil, nil)
	require.NoError(t, err)
	require.Len(t, summary, 2)
	require.Equal(t, "/v1/messages", summary[0].Endpoint)
	require.Equal(t, int64(2), summary[0].EventCount)
	require.Equal(t, int64(1), summary[0].BlockedCount)
	require.Equal(t, int64(3), summary[0].DetectedCount)

	stats, err := repo.GetStats(ctx, now.Add(-24*time.Hour))
	require.NoError(t, err)
	require.Equal(t, int64(3), stats.EventCount)
	require.Equal(t, int64(1), stats.BlockedCount)
	require.Equal(t, int64(3), stats.ByKind["API_KEY"])
	require.Equal(t, int64(2), stats.ByKind["EMAIL"])
}
