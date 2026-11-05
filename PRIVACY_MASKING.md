# 隐私脱敏功能文档

## 概述

Sub2API 的隐私脱敏功能可以在请求发送到上游 API 提供商之前自动检测并脱敏敏感信息（如 API 密钥、个人信息等），确保您的私密数据不会被发送到外部服务器。脱敏后的占位符会在响应返回时自动恢复，对最终用户透明。

### 主要特性

- ✅ **自动检测**：内置多种敏感信息检测规则（API 密钥、邮箱、电话等）
- ✅ **稳定占位符**：同一值总是生成相同的占位符，保证上下文一致性
- ✅ **流式支持**：完整支持流式响应（SSE）的脱敏和恢复
- ✅ **灵活动作**：对不同类型的敏感信息可配置不同动作（脱敏/警告/阻止）
- ✅ **自定义规则**：支持自定义正则表达式规则
- ✅ **白名单**：可配置永不脱敏的值
- ✅ **审计日志**：记录所有检测到的敏感信息（仅元数据，不存储原始值）

## 工作原理

### 脱敏流程

```
用户请求 → 检测敏感信息 → 替换为占位符 → 转发到上游
                                           ↓
用户收到响应 ← 恢复占位符 ← 上游响应
```

### 占位符格式

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

### 内存存储

原始值到占位符的映射**仅存在于单次请求的内存中**，请求结束即释放，不会写入磁盘、数据库或日志。这确保：
- 没有持久化的安全风险，也不会随运行时间无限增长
- 响应（包括 SSE 流）边写边恢复，不会缓冲整个响应

## 安装和配置

### 1. 数据库迁移

脱敏功能需要两个数据库表：
- `privacy_masking_config`：存储配置
- `privacy_masking_log`：存储审计日志

迁移文件 `backend/migrations/250_privacy_masking.sql`（建表）和 `251_privacy_masking_log_endpoint_index_notx.sql`（按接口查询的索引）会在启动时自动应用。配置保存后立即生效（多实例约 10 秒内同步）。

### 2. 后端配置

在 `main.go` 或路由配置中添加脱敏服务和处理器：

```go
import (
    "github.com/Wei-Shaw/sub2api/internal/repository"
    "github.com/Wei-Shaw/sub2api/internal/service"
    "github.com/Wei-Shaw/sub2api/internal/handler/admin"
)

// 创建服务
maskingService, _ := service.NewPrivacyMaskingService()
maskingRepo := repository.NewPrivacyMaskingRepository(db)
maskingHandler := admin.NewPrivacyMaskingHandler(maskingRepo, logger)

// 注册路由
adminGroup.GET("/privacy-masking/config", maskingHandler.GetGlobalConfig)
adminGroup.PUT("/privacy-masking/config", maskingHandler.UpdateGlobalConfig)
adminGroup.GET("/privacy-masking/logs", maskingHandler.ListLogs)
adminGroup.GET("/privacy-masking/stats", maskingHandler.GetStats)
```

### 3. 前端路由

在前端路由配置中添加：

```typescript
{
  path: '/admin/privacy-masking',
  name: 'AdminPrivacyMasking',
  component: () => import('@/views/admin/PrivacyMaskingView.vue'),
  meta: {
    title: '隐私脱敏',
    requiresAuth: true,
    requiresAdmin: true
  }
}
```

### 4. 导航菜单

在管理后台导航菜单中添加入口：

```vue
<RouterLink to="/admin/privacy-masking">
  隐私脱敏
</RouterLink>
```

## 使用指南

### 启用脱敏

1. 登录管理后台
2. 进入 "隐私脱敏" 页面
3. 打开 "启用隐私脱敏" 开关

### 基础配置

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

### 动作配置

对每种敏感信息类型，可以配置三种动作：

1. **脱敏（Mask）**：替换为占位符，请求正常发送
2. **警告（Warn）**：记录日志但不脱敏，请求正常发送
3. **阻止（Block）**：检测到后直接拒绝请求

### 白名单

对于某些已知安全的值（如测试密钥），可以添加到白名单：

```
sk-test-1234567890
test@example.com
+1234567890
```

白名单中的值会被精确匹配（不区分大小写），永远不会被脱敏。

### 自定义规则

使用正则表达式定义自定义检测规则：

```
模式：secret-[a-zA-Z0-9]+
动作：阻止
```

## Docker 部署

### 完整部署步骤

1. **拉取最新代码**

```bash
cd /path/to/sub2api
git pull origin main
```

2. **重新构建镜像**

```bash
docker-compose down
docker-compose build
```

3. **启动服务**

```bash
docker-compose up -d
```

4. **检查迁移**

```bash
docker-compose logs backend | grep migration
```

确认 `250_privacy_masking.sql` 迁移已应用。

5. **访问管理界面**

打开浏览器访问：`https://your-domain.com/admin/privacy-masking`

### 仅更新后端

如果只修改了后端代码：

```bash
docker-compose stop backend
docker-compose build backend
docker-compose up -d backend
```

### 仅更新前端

如果只修改了前端代码：

```bash
docker-compose stop frontend
docker-compose build frontend
docker-compose up -d frontend
```

### 环境变量（可选）

在 `docker-compose.yml` 或 `.env` 中添加：

```yaml
environment:
  - MASKING_ENABLED=true
  - MASKING_LOG_RETENTION_DAYS=180
```

## 监控和审计

### 查看脱敏日志

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

### 日志保留

目前没有自动清理，`privacy_masking_log` 会持续增长，可按 `detected_at` 定期手动清理。

### API 查询

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

## 性能影响

### 计算开销

- 正则匹配：每个规则约 0.1-1ms（取决于文本长度）
- 总开销：通常 < 10ms

### 内存使用

- 占位符映射只在单次请求内存在，请求结束即释放

### 优化建议

1. **限制自定义规则数量**：过多的正则规则会增加延迟
2. **使用白名单**：对频繁出现的已知安全值添加白名单
3. **定期清理日志**：`privacy_masking_log` 目前没有自动清理

## 安全考虑

### 安全特性

✅ **不存储原始值**：只在内存中临时保存，不写入磁盘
✅ **审计日志**：仅存储元数据（类型、数量、前缀）
✅ **默认阻止**：如果脱敏失败，默认拒绝请求
✅ **HMAC 保护**：占位符使用密钥哈希，不可预测

### 风险和限制

⚠️ **跨事件拆分**：模型若把一个占位符拆到多个 SSE 事件里输出，该占位符无法恢复
⚠️ **非 JSON 请求**：multipart、二进制等非 JSON 请求体不检测
⚠️ **通用长串规则**：32 位以上字母数字串（如哈希）也会被视为 API_KEY 脱敏；紧邻 `+ / =` 的 base64 片段会被跳过
⚠️ **上游缓存**：如果上游缓存了占位符响应，需要清除缓存
⚠️ **规则完整性**：内置规则无法覆盖所有情况，可能有漏检

### 最佳实践

1. **测试环境验证**：先在测试环境启用，验证不影响业务
2. **逐步启用**：先启用 "仅警告" 模式，观察一段时间后再启用脱敏
3. **监控日志**：定期检查审计日志，调整规则
4. **白名单管理**：及时更新白名单，避免误报

## 故障排除

### 问题：启用后请求失败

**可能原因**：脱敏后的 JSON 格式损坏

**解决方案**：
1. 检查自定义规则是否过于宽泛
2. 将可疑类型的动作改为 "警告"
3. 查看日志确认被阻止的原因

### 问题：响应中占位符未恢复

**可能原因**：
1. 模型把占位符拆到了多个流式事件中
2. 模型改写了占位符（大小写、空格）
3. 响应来自上游缓存

**解决方案**：
1. 重新发送请求
2. 清除上游缓存
3. 检查响应处理逻辑

### 问题：性能下降

**可能原因**：自定义规则过多或过于复杂

**解决方案**：
1. 减少自定义规则数量
2. 优化正则表达式
3. 使用白名单减少匹配次数

## 技术实现

### 架构组件

```
┌─────────────────────────────────────────┐
│           Client Request                 │
└───────────────┬─────────────────────────┘
                │
                ↓
┌─────────────────────────────────────────┐
│      Privacy Masking Middleware          │
│  - Load Config                           │
│  - Detect Sensitive Data                 │
│  - Replace with Placeholders             │
│  - Store Mapping (Memory Only)           │
└───────────────┬─────────────────────────┘
                │
                ↓
┌─────────────────────────────────────────┐
│      Forward to Upstream API             │
└───────────────┬─────────────────────────┘
                │
                ↓
┌─────────────────────────────────────────┐
│      Upstream Response                   │
└───────────────┬─────────────────────────┘
                │
                ↓
┌─────────────────────────────────────────┐
│      Privacy Masking Middleware          │
│  - Restore Placeholders                  │
│  - Return to Client                      │
└───────────────┬─────────────────────────┘
                │
                ↓
┌─────────────────────────────────────────┐
│           Client Response                │
└─────────────────────────────────────────┘
```

### 关键文件

**后端**：
- `backend/internal/service/privacy_masking.go` - 核心脱敏服务
- `backend/internal/service/privacy_masking_middleware.go` - 中间件
- `backend/internal/repository/privacy_masking_repo.go` - 数据访问
- `backend/internal/handler/admin/privacy_masking_handler.go` - API 处理器
- `backend/migrations/250_privacy_masking.sql` - 数据库迁移

**前端**：
- `frontend/src/views/admin/PrivacyMaskingView.vue` - 管理界面

### API 端点

- `GET /api/v1/admin/privacy-masking/config` - 获取配置
- `PUT /api/v1/admin/privacy-masking/config` - 更新配置（校验动作与正则，保存即生效）
- `GET /api/v1/admin/privacy-masking/logs` - 分页查询日志（支持接口、类型、结果、时间等过滤）
- `GET /api/v1/admin/privacy-masking/logs/endpoints` - 按 API 接口聚合
- `GET /api/v1/admin/privacy-masking/stats` - 统计概览

## 参考

### 灵感来源

本功能参考了 [yetone/magpie](https://github.com/yetone/magpie) 项目的隐私脱敏实现，感谢开源社区的贡献。

### 许可证

- Sub2API: LGPL-3.0
- Magpie: MIT

## 支持

如有问题或建议，请提交 Issue 或 Pull Request。
