-- 智能脱敏（Presidio 旁路）配置
-- Smart detection (Presidio sidecar) settings; disabled by default so existing behavior is unchanged.

ALTER TABLE privacy_masking_config
    ADD COLUMN IF NOT EXISTS smart_masking_enabled BOOLEAN NOT NULL DEFAULT false,
    -- 旁路服务地址；为空时使用环境变量 PRIVACY_MASKING_SMART_URL
    ADD COLUMN IF NOT EXISTS smart_sidecar_url VARCHAR(512) NOT NULL DEFAULT '',
    -- 单次请求调用旁路的硬超时（毫秒），超时后仅使用规则脱敏
    ADD COLUMN IF NOT EXISTS smart_timeout_ms INTEGER NOT NULL DEFAULT 80,
    -- 实体置信度阈值（0~1），低于阈值的结果丢弃
    ADD COLUMN IF NOT EXISTS smart_min_score DOUBLE PRECISION NOT NULL DEFAULT 0.6,
    -- 启用的实体类型，如 ["PERSON","LOCATION","ORG"]；动作沿用 kind_actions
    ADD COLUMN IF NOT EXISTS smart_entities JSONB NOT NULL DEFAULT '["PERSON","LOCATION","ORG"]';
