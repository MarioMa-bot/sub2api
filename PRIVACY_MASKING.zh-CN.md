# 隐私脱敏功能文档 | Privacy Masking Documentation

[English](#english-version) | [中文](#中文版本)

---

## 中文版本

### 概述

Sub2API 的隐私脱敏功能可以在请求发送到上游 API 提供商之前自动检测并脱敏敏感信息（如 API 密钥、个人信息等），确保您的私密数据不会被发送到外部服务器。脱敏后的占位符会在响应返回时自动恢复，对最终用户透明。

### 主要特性

- ✅ **自动检测**：内置多种敏感信息检测规则（API 密钥、邮箱、电话等）
- ✅ **稳定占位符**：同一值总是生成相同的占位符，保证上下文一致性
- ✅ **流式支持**：完整支持流式响应（SSE）的脱敏和恢复
- ✅ **灵活动作**：对不同类型的敏感信息可配置不同动作（脱敏/警告/阻止）
- ✅ **自定义规则**：支持自定义正则表达式规则
- ✅ **白名单**：可配置永不脱敏的值
- ✅ **审计日志**：记录所有检测到的敏感信息（仅元数据，不存储原始值）

### 工作原理

```
用户请求 → 检测敏感信息 → 替换为占位符 → 转发到上游
                                           ↓
用户收到响应 ← 恢复占位符 ← 上游响应
```

#### 占位符格式

```
{{KIND_hash}}
```

例如：
- API 密钥：`{{API_KEY_a3b4c5d6}}`
- 邮箱：`{{EMAIL_x7y8z9w0}}`
- 电话：`{{PHONE_m1n2o3p4}}`

占位符使用 HMAC-SHA256 生成，保证：
1. **稳定性**：同一个值总是生成相同的占位符
2. **唯一性**：不同的值生成不同的占位符
3. **安全性**：无法从占位符反推原始值

#### 内存存储

原始值到占位符的映射**仅存在于单次请求的内存中**，请求结束即释放，不会写入磁盘、数据库或日志。这确保：
- 没有持久化的安全风险，也不会随运行时间无限增长
- 响应（包括 SSE 流）边写边恢复，不会缓冲整个响应

### 安装和配置

#### 1. 数据库迁移

脱敏功能需要两个数据库表：
- `privacy_masking_config`：存储配置
- `privacy_masking_log`：存储审计日志

迁移文件 `backend/migrations/250_privacy_masking.sql`（建表）和 `251_privacy_masking_log_endpoint_index_notx.sql`（按接口查询的索引）会在启动时自动应用。配置保存后立即生效（多实例约 10 秒内同步）。

#### 2. 启用脱敏

1. 登录管理后台
2. 进入 "隐私脱敏" 页面
3. 打开 "启用隐私脱敏" 开关

#### 3. 配置规则

**脱敏密钥和令牌**：检测并脱敏
- OpenAI API 密钥（`sk-...`）
- Anthropic API 密钥（`sk-ant-...`）
- GitHub Token（`ghp_...`, `gho_...`）
- JWT Token
- 私钥（PEM 格式）
- 密码字段

**脱敏个人信息**：检测并脱敏
- 邮箱地址
- 电话号码（美国、中国格式）
- 信用卡号
- 身份证号

#### 4. 动作配置

对每种敏感信息类型，可以配置三种动作：

1. **脱敏（Mask）**：替换为占位符，请求正常发送
2. **警告（Warn）**：记录日志但不脱敏，请求正常发送
3. **阻止（Block）**：检测到后直接拒绝请求

### Docker 部署

#### 完整部署步骤

```bash
# 1. 拉取最新代码
cd /path/to/sub2api
git pull origin main

# 2. 重新构建镜像
docker-compose down
docker-compose build

# 3. 启动服务
docker-compose up -d

# 4. 检查迁移
docker-compose logs backend | grep migration
```

确认 `250_privacy_masking.sql` 与 `251_privacy_masking_log_endpoint_index_notx.sql` 迁移已应用。

#### 5. 访问管理界面

打开浏览器访问：`https://your-domain.com/admin/privacy-masking`

### 监控和审计

#### 查看脱敏日志

管理后台 → 隐私脱敏 → **「脱敏日志」** 标签页（`/admin/privacy-masking?tab=logs`）：

- **统计卡片**：所选时间范围内的脱敏事件数、被阻止的请求数、已脱敏/仅警告的值数量
- **按 API 接口统计**：每个接口（如 `/v1/messages`、`/v1/chat/completions`、`/v1beta/models/{model}:generateContent`）的事件数、阻止数、命中值数和最近一次时间；点击某一行即按该接口过滤
- **日志列表**：时间、接口 / 模型、调用方（用户 ID、API Key ID、分组 ID）、命中类型及数量、处理结果（脱敏/警告/阻止数量）、是否阻止、请求 ID
- **过滤条件**：API 接口、类型、结果（已阻止/已通过）、时间范围（含自定义）、模型、用户 ID、API Key ID；支持分页

日志只记录元数据，**绝不包含原始敏感值**；值示例只保留长度，API_KEY/TOKEN 类型额外保留前 4 个字符（如 `sk-a...`，属于公开的格式前缀）。

记录规则：
- 网关所有入口（`/v1`、`/v1beta`、根路径别名、`/backend-api/codex`、`/antigravity`）的 POST/PUT/PATCH JSON 请求体都会检测
- 命中任意规则（脱敏、警告、阻止）即写一条日志；被阻止的请求也会记录
- 日志在请求转发前异步写入，上游失败不影响记录；未启用脱敏时不产生日志
- 接口名使用实际路径；带路径参数的路由使用路由模板，Gemini 的模型名归一为 `{model}`

#### 日志保留

目前没有自动清理，`privacy_masking_log` 会持续增长，可按 `detected_at` 定期手动清理。

#### API 查询

所有接口位于 `/api/v1/admin/privacy-masking`，返回统一的 `{code, message, data}` 结构，需管理员登录。

```bash
# 分页查询日志（page / page_size，page_size 最大 200）
GET /api/v1/admin/privacy-masking/logs?page=1&page_size=20

# 过滤：endpoint、kind、model（模糊）、user_id、api_key_id、group_id、blocked、start_time / end_time（RFC3339）
GET /api/v1/admin/privacy-masking/logs?endpoint=/v1/messages&kind=API_KEY&blocked=true&start_time=2026-11-01T00:00:00Z

# 按 API 接口聚合（可选 start_time / end_time）
GET /api/v1/admin/privacy-masking/logs/endpoints

# 统计概览（since 默认 24 小时前）
GET /api/v1/admin/privacy-masking/stats?since=2026-11-01T00:00:00Z
```

### 安全考虑

#### 安全特性

✅ **不存储原始值**：只在内存中临时保存，不写入磁盘  
✅ **审计日志**：仅存储元数据（类型、数量、前缀）  
✅ **默认阻止**：如果脱敏失败，默认拒绝请求  
✅ **HMAC 保护**：占位符使用密钥哈希，不可预测

#### 最佳实践

1. **测试环境验证**：先在测试环境启用，验证不影响业务
2. **逐步启用**：先启用 "仅警告" 模式，观察一段时间后再启用脱敏
3. **监控日志**：定期检查审计日志，调整规则
4. **白名单管理**：及时更新白名单，避免误报

### 故障排除

#### 问题：启用后请求失败

**可能原因**：脱敏后的 JSON 格式损坏

**解决方案**：
1. 检查自定义规则是否过于宽泛
2. 将可疑类型的动作改为 "警告"
3. 查看日志确认被阻止的原因

#### 问题：响应中占位符未恢复

**可能原因**：
1. 服务重启导致映射丢失
2. 响应来自上游缓存

**解决方案**：
1. 重新发送请求
2. 清除上游缓存
3. 检查响应处理逻辑

---

## English Version

### Overview

Sub2API's privacy masking feature automatically detects and masks sensitive information (such as API keys, personal data) before requests are sent to upstream API providers, ensuring your private data is never sent to external servers. Masked placeholders are automatically restored when responses return, transparent to end users.

### Key Features

- ✅ **Auto-detection**: Built-in detection rules for various sensitive data (API keys, emails, phones, etc.)
- ✅ **Stable placeholders**: Same value always generates the same placeholder, ensuring context consistency
- ✅ **Streaming support**: Full support for masking and restoring SSE streaming responses
- ✅ **Flexible actions**: Configure different actions for different types of sensitive data (Mask/Warn/Block)
- ✅ **Custom rules**: Support custom regex patterns
- ✅ **Allow list**: Configure values that should never be masked
- ✅ **Audit logs**: Record all detected sensitive data (metadata only, no raw values)

### How It Works

```
User Request → Detect Sensitive Data → Replace with Placeholders → Forward to Upstream
                                                                           ↓
User Receives ← Restore Placeholders ← Upstream Response
```

#### Placeholder Format

```
{{KIND_hash}}
```

Examples:
- API Key: `{{API_KEY_a3b4c5d6}}`
- Email: `{{EMAIL_x7y8z9w0}}`
- Phone: `{{PHONE_m1n2o3p4}}`

Placeholders are generated using HMAC-SHA256, ensuring:
1. **Stability**: Same value → same placeholder
2. **Uniqueness**: Different values → different placeholders
3. **Security**: Cannot reverse-engineer original value

#### Memory-Only Storage

Original value ↔ placeholder mappings live **in memory for the lifetime of a single request only**; they are never written to disk, the database or logs. This ensures:
- No persistent security risk, and no unbounded growth over time
- Responses (including SSE streams) are restored as they are written, without buffering the whole body

### Installation and Configuration

#### 1. Database Migration

The masking feature requires two database tables:
- `privacy_masking_config`: Store configuration
- `privacy_masking_log`: Store audit logs

Migrations `backend/migrations/250_privacy_masking.sql` (tables) and `251_privacy_masking_log_endpoint_index_notx.sql` (per-endpoint index) are applied automatically on startup. Saved config takes effect immediately (within ~10s on other instances).

#### 2. Enable Masking

1. Login to admin dashboard
2. Navigate to "Privacy Masking" page
3. Toggle "Enable Privacy Masking" switch

#### 3. Configure Rules

**Mask Secrets**: Detect and mask
- OpenAI API keys (`sk-...`)
- Anthropic API keys (`sk-ant-...`)
- GitHub Tokens (`ghp_...`, `gho_...`)
- JWT Tokens
- Private keys (PEM format)
- Password fields

**Mask Personal Data**: Detect and mask
- Email addresses
- Phone numbers (US, China formats)
- Credit card numbers
- ID numbers

#### 4. Action Configuration

For each type of sensitive data, configure one of three actions:

1. **Mask**: Replace with placeholder, request proceeds normally
2. **Warn**: Log but don't mask, request proceeds normally
3. **Block**: Reject request immediately when detected

### Docker Deployment

#### Full Deployment Steps

```bash
# 1. Pull latest changes
cd /path/to/sub2api
git pull origin main

# 2. Rebuild images
docker-compose down
docker-compose build

# 3. Start services
docker-compose up -d

# 4. Verify migration
docker-compose logs backend | grep migration
```

Confirm `250_privacy_masking.sql` migration is applied.

#### 5. Access Admin UI

Open browser: `https://your-domain.com/admin/privacy-masking`

### Monitoring and Auditing

#### View Masking Logs

Admin console → Privacy Masking → **"Masking Logs"** tab (`/admin/privacy-masking?tab=logs`):

- **Stat cards**: masking events, blocked requests, values masked / warned in the selected time range
- **By API endpoint**: events, blocked, values hit and last seen per endpoint (e.g. `/v1/messages`, `/v1/chat/completions`, `/v1beta/models/{model}:generateContent`); click a row to filter by it
- **Log table**: time, endpoint / model, caller (user ID, API key ID, group ID), kinds with counts, handling (masked / warned / blocked counts), blocked status, request ID
- **Filters**: endpoint, kind, result (blocked / passed), time range (incl. custom), model, user ID, API key ID; paginated

Logs hold metadata only and **never contain raw values**. Samples keep only the length, plus the first 4 characters for API_KEY / TOKEN (public format prefixes such as `sk-a...`).

What gets logged:
- POST/PUT/PATCH JSON bodies on every gateway entry (`/v1`, `/v1beta`, root aliases, `/backend-api/codex`, `/antigravity`)
- One row per request that hits any rule (mask, warn or block), including blocked requests
- Written asynchronously before forwarding, so upstream failures do not drop the row; nothing is logged while masking is disabled
- The endpoint is the request path; routes with path params use the route template, and Gemini model names are normalized to `{model}`

There is no automatic retention yet; prune `privacy_masking_log` by `detected_at` if needed.

#### API Queries

All endpoints live under `/api/v1/admin/privacy-masking`, require an admin session and return the standard `{code, message, data}` envelope.

```bash
# Paginated logs (page / page_size, page_size max 200)
GET /api/v1/admin/privacy-masking/logs?page=1&page_size=20

# Filters: endpoint, kind, model (substring), user_id, api_key_id, group_id, blocked, start_time / end_time (RFC3339)
GET /api/v1/admin/privacy-masking/logs?endpoint=/v1/messages&kind=API_KEY&blocked=true&start_time=2026-11-01T00:00:00Z

# Aggregated per API endpoint (optional start_time / end_time)
GET /api/v1/admin/privacy-masking/logs/endpoints

# Overview stats (since defaults to 24h ago)
GET /api/v1/admin/privacy-masking/stats?since=2026-11-01T00:00:00Z
```

### Security Considerations

#### Security Features

✅ **No raw value storage**: Only in memory, never persisted  
✅ **Audit logs**: Store metadata only (type, count, prefix)  
✅ **Fail-safe**: Block request if masking fails  
✅ **HMAC protection**: Placeholders use keyed hash, unpredictable

#### Best Practices

1. **Test first**: Enable in test environment before production
2. **Start with warnings**: Use "Warn" action initially, monitor logs
3. **Monitor logs**: Regularly check audit logs and adjust rules
4. **Manage allow list**: Keep allow list updated to avoid false positives

### Troubleshooting

#### Issue: Requests fail after enabling

**Possible cause**: Masked JSON format corrupted

**Solutions**:
1. Check if custom rules are too broad
2. Change suspicious types to "Warn" action
3. Check logs to confirm blocking reason

#### Issue: Placeholders not restored in response

**Possible causes**:
1. Service restart caused mapping loss
2. Response came from upstream cache

**Solutions**:
1. Resend request
2. Clear upstream cache
3. Check response handling logic

---

### 参考 | References

本功能参考了 [yetone/magpie](https://github.com/yetone/magpie) 项目的隐私脱敏实现。  
This feature is inspired by [yetone/magpie](https://github.com/yetone/magpie)'s privacy masking implementation.

### 许可证 | License

- Sub2API: LGPL-3.0
- Magpie: MIT

### 支持 | Support

如有问题或建议，请提交 Issue 或 Pull Request。  
For questions or suggestions, please submit an Issue or Pull Request.
