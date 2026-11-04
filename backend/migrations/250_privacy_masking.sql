-- Privacy Masking Configuration and Logging Tables
-- 隐私脱敏配置和日志表

-- 全局隐私脱敏配置表
CREATE TABLE IF NOT EXISTS privacy_masking_config (
    id BIGSERIAL PRIMARY KEY,
    
    -- 基础配置
    enabled BOOLEAN NOT NULL DEFAULT false,
    mask_secrets BOOLEAN NOT NULL DEFAULT true,  -- 脱敏密钥、token等敏感信息
    mask_personal BOOLEAN NOT NULL DEFAULT false, -- 脱敏个人信息（邮箱、电话等）
    
    -- 细粒度控制：哪些类型的敏感信息需要脱敏
    -- JSONB 格式: {"api_key": true, "email": true, "phone": false, ...}
    masked_kinds JSONB NOT NULL DEFAULT '{}',
    
    -- 自定义关键词和正则表达式
    -- JSONB 格式: [{"pattern": "sk-.*", "kind": "api_key", "action": "mask"}, ...]
    custom_rules JSONB NOT NULL DEFAULT '[]',
    
    -- 白名单：永不脱敏的值（精确匹配，不区分大小写）
    -- JSONB 格式: ["example.com", "test-key", ...]
    allow_list JSONB NOT NULL DEFAULT '[]',
    
    -- 每种类型的动作配置
    -- JSONB 格式: {"api_key": "mask", "email": "warn", "custom_secret": "block", ...}
    -- 可选值: "mask" (脱敏), "warn" (警告但不脱敏), "block" (阻止请求)
    kind_actions JSONB NOT NULL DEFAULT '{}',
    
    -- 分组级别覆盖（可选功能，初期可以不实现）
    -- NULL 表示全局配置，非 NULL 表示特定分组的覆盖配置
    group_id BIGINT DEFAULT NULL,
    
    -- 审计字段
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by_user_id BIGINT,
    
    -- 索引
    UNIQUE(group_id) -- 每个分组（或全局）只有一个配置
);

CREATE INDEX IF NOT EXISTS idx_privacy_masking_config_enabled ON privacy_masking_config(enabled);
CREATE INDEX IF NOT EXISTS idx_privacy_masking_config_group_id ON privacy_masking_config(group_id) WHERE group_id IS NOT NULL;

-- 插入默认全局配置（如果不存在）
INSERT INTO privacy_masking_config (enabled, mask_secrets, mask_personal, group_id)
VALUES (false, true, false, NULL)
ON CONFLICT (group_id) DO NOTHING;

-- 隐私脱敏日志表（记录每次检测到的敏感信息，但不存储原始值）
CREATE TABLE IF NOT EXISTS privacy_masking_log (
    id BIGSERIAL PRIMARY KEY,
    
    -- 请求上下文
    request_id VARCHAR(255),
    user_id BIGINT NOT NULL,
    api_key_id BIGINT,
    group_id BIGINT,
    
    -- 检测信息
    detected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    kinds JSONB NOT NULL, -- 检测到的类型及数量 {"api_key": 2, "email": 1}
    actions JSONB NOT NULL, -- 每个类型采取的动作 {"api_key": "mask", "email": "warn"}
    
    -- 脱敏统计
    total_count INTEGER NOT NULL DEFAULT 0, -- 总共检测到的敏感值数量
    masked_count INTEGER NOT NULL DEFAULT 0, -- 实际脱敏的数量
    warned_count INTEGER NOT NULL DEFAULT 0, -- 仅警告的数量
    blocked_count INTEGER NOT NULL DEFAULT 0, -- 导致阻止的数量
    
    -- 请求是否被阻止
    request_blocked BOOLEAN NOT NULL DEFAULT false,
    
    -- 示例前缀（用于审计，不存储完整值）
    -- JSONB 格式: [{"kind": "api_key", "prefix": "sk-...", "length": 48}, ...]
    value_samples JSONB,
    
    -- 端点和模型信息
    endpoint VARCHAR(255),
    model VARCHAR(255),
    
    -- 索引
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_privacy_masking_log_user_id ON privacy_masking_log(user_id);
CREATE INDEX IF NOT EXISTS idx_privacy_masking_log_detected_at ON privacy_masking_log(detected_at);
CREATE INDEX IF NOT EXISTS idx_privacy_masking_log_request_blocked ON privacy_masking_log(request_blocked);
CREATE INDEX IF NOT EXISTS idx_privacy_masking_log_group_id ON privacy_masking_log(group_id) WHERE group_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_privacy_masking_log_request_id ON privacy_masking_log(request_id) WHERE request_id IS NOT NULL;

-- 自动更新 updated_at 触发器
CREATE OR REPLACE FUNCTION update_privacy_masking_config_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_privacy_masking_config_updated_at
    BEFORE UPDATE ON privacy_masking_config
    FOR EACH ROW
    EXECUTE FUNCTION update_privacy_masking_config_updated_at();
