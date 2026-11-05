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

### 智能脱敏（Presidio 旁路，可选）/ Smart detection

规则脱敏（L1，正则，进程内 CPU）之外，可以再开启 L2 智能检测：后端把请求中**消息正文类字段**（`content`、`text`、`system`、`prompt`、`input`、`instructions`、`query` 下的字符串）送到 Presidio 旁路服务，识别人名、地点、机构等实体，然后用与规则相同的 `{{KIND_hash}}` 占位符替换，并在响应 / SSE 中恢复。`model`、`role`、工具名等字段不会送检，也不会被改写。

- 实体类型 → 占位符类型：`PERSON`、`LOCATION`、`ORG`（默认启用），以及可选的 `NRP`（国籍/宗教/政治团体）、`DATE_TIME`（误报较多，默认关闭）。
- 动作沿用「类型与动作」：在智能检测区块为每种实体选择 脱敏 / 仅警告 / 阻止请求。
- 日志与规则命中写在同一张 `privacy_masking_log` 表里，`kinds` 中会出现 `PERSON` 等类型，**只记录类型、数量和长度，不记录原文**（这些类型不保留前缀）。
- 规则命中与智能命中重叠时，规则结果优先。

#### 降级行为

- 旁路的每次调用都有**硬超时**（默认 80 ms，可配 10~5000 ms）。超时、连接失败、HTTP 非 200、响应无法解析、地址未配置时，本次请求**静默降级为仅规则脱敏**，请求照常转发，不会因 NER 失败返回错误。
- 降级时后端每 30 秒最多打一条 `smart detection unavailable, falling back to rules only` 警告日志。
- 与规则脱敏不同：配置读取失败仍然按原逻辑返回 503（失败即阻断）；只有 NER 本身的失败才会降级。
- 关闭「智能检测」开关时完全不调用旁路，行为与之前一致。

#### 本地启动

**方式一：docker compose（推荐）**

`deploy/docker-compose.dev.yml` 已包含 `privacy-ner` 服务，后端默认通过 `PRIVACY_MASKING_SMART_URL=http://privacy-ner:5002` 访问：

```bash
cd deploy
docker compose -f docker-compose.dev.yml up --build -d
docker compose -f docker-compose.dev.yml logs -f privacy-ner   # 看到 "privacy-ner listening" 即就绪（首次加载模型约 10~20 秒）
```

然后在 管理后台 → 隐私脱敏 → 规则配置 → 「智能检测 / Smart detection」：打开开关，地址留空（使用环境变量）或填写 `http://privacy-ner:5002`，点「检测连接」，保存。

生产 compose（`deploy/docker-compose.yml`）中该服务放在 `smart-masking` profile 下，默认不启动：

```bash
cd deploy
docker compose --profile smart-masking up -d --build
```

注意：`docker-compose.yml` 默认使用上游镜像 `weishaw/sub2api`，只有使用本 fork 构建的镜像才包含智能脱敏代码。

**方式二：直接运行 Python（无 Docker）**

```bash
cd deploy/presidio
python3 -m venv .venv && . .venv/bin/activate
pip install -r requirements.txt
python app.py                      # 默认监听 0.0.0.0:5002
curl -s localhost:5002/health
curl -s -X POST localhost:5002/analyze -H 'content-type: application/json' \
  -d '{"text":"Ask John Smith in Seattle. 李明住在上海。"}'
```

后端启动时设置 `PRIVACY_MASKING_SMART_URL=http://127.0.0.1:5002`，或在管理后台填写地址。

#### 旁路 API

- `GET /health` → `{"status":"ok","languages":["en","zh"]}`
- `POST /analyze`，请求 `{"text": "...", "entities": ["PERSON","LOCATION","ORG"], "min_score": 0.6}`（`entities`、`min_score`、`language` 可选），响应 `[{"start":0,"end":10,"kind":"PERSON","score":0.85}]`。`start` / `end` 为 Unicode 码点偏移（左闭右开）。
- 旁路只返回位置与类型，不记录、不保存请求文本。

#### 配置项

| 位置 | 名称 | 默认值 | 说明 |
|---|---|---|---|
| 管理后台 | 智能检测开关 `smart_masking_enabled` | 关闭 | 总开关；保存后立即生效（其他实例 10 秒内） |
| 管理后台 | 旁路服务地址 `smart_sidecar_url` | 空 | 为空时用环境变量 `PRIVACY_MASKING_SMART_URL` |
| 管理后台 | 超时 `smart_timeout_ms` | 80 | 单次请求硬超时（10~5000） |
| 管理后台 | 置信度阈值 `smart_min_score` | 0.6 | 0~1，低于阈值的实体忽略 |
| 管理后台 | 实体类型 `smart_entities` | PERSON, LOCATION, ORG | 动作在同一区块设置 |
| 后端环境变量 | `PRIVACY_MASKING_SMART_URL` | compose 中为 `http://privacy-ner:5002` | 后台地址留空时使用 |
| 旁路环境变量 | `PRIVACY_NER_LANGUAGES` | `en,zh` | 加载的语言模型 |
| 旁路环境变量 | `PRIVACY_NER_MIN_SCORE` | 0.5 | 旁路侧最低阈值（与后台阈值取较大者） |
| 旁路环境变量 | `PRIVACY_NER_MAX_TEXT_CHARS` | 65536 | 单次分析的最大字符数 |
| 旁路环境变量 | `PRIVACY_NER_PORT` | 5002 | 监听端口 |
| compose 构建参数 | `PRIVACY_NER_PIP_INDEX_URL` | PyPI 官方源 | 国内可改为镜像源；spaCy 模型包仍从 GitHub Releases 下载 |

配置保存在 `privacy_masking_config` 表（迁移 `252_privacy_masking_smart_detection.sql` 新增列），管理员 API 新增 `POST /api/v1/admin/privacy-masking/smart/check`（检测旁路连通性，`url` 为空时检测已保存地址）。

#### 模型与限制

- 使用 Presidio Analyzer 2.2 + spaCy 小模型 `en_core_web_sm`、`zh_core_web_sm`（纯 CPU，镜像约 850 MB，常驻内存约 450 MB）。中英混排文本会两种模型各跑一次再合并，单次约 5~15 ms。
- 小模型准确率有限：中文人名、少见地名、英文小写人名都可能漏检；也可能把普通词误判为机构。需要更高准确率时可把 `PRIVACY_NER_EN_MODEL` / `PRIVACY_NER_ZH_MODEL` 换成 `*_lg` 模型（需自行修改 `requirements.txt`，延迟和内存会上升，相应调大超时）。
- 每次请求最多送检约 32 KB 文本，超出时优先保留最新的消息；更早的长对话历史只做规则脱敏。
- 实体在原始 JSON 中按「UTF-8 原文」和「`\uXXXX` 转义」两种编码查找；其他罕见编码形式会漏掉（只影响 L2，不影响规则）。
- 英文实体要求两侧不是字母数字（避免 `Ann` 命中 `Annual`）；中文实体按子串匹配。
- 旁路超时会直接放弃这次的智能结果，不会重试。

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

### Smart Detection (Presidio sidecar, optional)

On top of rule masking (L1, regex, in-process), you can enable L2 smart detection. The backend sends strings under **message text fields** (`content`, `text`, `system`, `prompt`, `input`, `instructions`, `query`) to a Presidio sidecar. Detected person / location / organization entities are replaced with the same `{{KIND_hash}}` placeholders and restored in responses and SSE. Fields such as `model`, `role` or tool names are never sent or rewritten.

- Kinds: `PERSON`, `LOCATION`, `ORG` (enabled by default), plus optional `NRP` and `DATE_TIME` (noisy, off by default). Actions (mask / warn / block) are set per kind in the same section.
- Logs go to the same `privacy_masking_log` table with kinds such as `PERSON`. **Only kinds, counts and lengths are stored, never the text.**
- When a rule hit and a smart hit overlap, the rule wins.

**Degradation.** Every sidecar call has a hard timeout (default 80 ms, 10–5000 ms). On timeout, connection error, non-200, bad response or missing URL, that request silently falls back to rules-only and is forwarded normally. NER failures never fail a request. A warning is logged at most every 30 s. With the switch off, the sidecar is never called and behavior is unchanged.

**Run locally with compose:**

```bash
cd deploy
docker compose -f docker-compose.dev.yml up --build -d
docker compose -f docker-compose.dev.yml logs -f privacy-ner   # ready when "privacy-ner listening" appears
```

Then go to Admin → Privacy Masking → Rules → "Smart detection / 智能检测". Turn it on, leave the URL empty (uses `PRIVACY_MASKING_SMART_URL=http://privacy-ner:5002` from compose) and click "Test connection", then save. In production `deploy/docker-compose.yml` the sidecar sits behind the `smart-masking` profile: `docker compose --profile smart-masking up -d --build`. The default image `weishaw/sub2api` is upstream and does not contain this fork's code.

**Without Docker:** `cd deploy/presidio && python3 -m venv .venv && . .venv/bin/activate && pip install -r requirements.txt && python app.py`. Then start the backend with `PRIVACY_MASKING_SMART_URL=http://127.0.0.1:5002`.

**Sidecar API:** `GET /health`; `POST /analyze {"text": "...", "entities": [...], "min_score": 0.6}` → `[{"start", "end", "kind", "score"}]`. Offsets are Unicode code points (end exclusive). The sidecar never logs or stores text.

**Settings:** admin UI fields `smart_masking_enabled`, `smart_sidecar_url`, `smart_timeout_ms` (80), `smart_min_score` (0.6) and `smart_entities`, added by migration `252_privacy_masking_smart_detection.sql`. Backend env var `PRIVACY_MASKING_SMART_URL`. Sidecar env vars `PRIVACY_NER_LANGUAGES` (`en,zh`), `PRIVACY_NER_MIN_SCORE` (0.5), `PRIVACY_NER_MAX_TEXT_CHARS` (65536) and `PRIVACY_NER_PORT` (5002). Compose build arg `PRIVACY_NER_PIP_INDEX_URL`. New admin API: `POST /api/v1/admin/privacy-masking/smart/check`.

**Limits:**
- spaCy small models (`en_core_web_sm`, `zh_core_web_sm`) on CPU: about 850 MB image and ~450 MB RAM, 5–15 ms per call. Mixed zh/en text runs both models.
- Small models miss some names (especially Chinese ones) and may produce false positives. Switch to `*_lg` models for accuracy, at the cost of latency and memory.
- At most about 32 KB of text per request is scanned, newest messages first; older history gets rule masking only.
- Entities are matched in the raw JSON as UTF-8 text or `\uXXXX` escapes only.

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
