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

原始值到占位符的映射**仅存储在内存中**，不会写入磁盘或数据库。这确保：
- 重启后映射丢失（下次请求会重新生成）
- 没有持久化的安全风险
- 性能优秀

## 安装和配置

### 1. 数据库迁移

脱敏功能需要两个数据库表：
- `privacy_masking_config`：存储配置
- `privacy_masking_log`：存储审计日志

迁移文件已包含在 `backend/migrations/250_privacy_masking.sql`，运行应用时会自动应用。

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

在管理界面的 "最近的脱敏日志" 部分可以看到：
- 检测时间
- 检测到的类型和数量
- 采取的动作
- 请求是否被阻止

### 日志保留

- 命中日志默认保留 180 天
- 未命中日志默认保留 3 天
- 可通过配置调整保留期限

### API 查询

通过 API 查询脱敏日志：

```bash
# 获取所有日志
GET /api/admin/privacy-masking/logs

# 过滤参数
GET /api/admin/privacy-masking/logs?user_id=123&blocked=true&limit=100
```

## 性能影响

### 计算开销

- 正则匹配：每个规则约 0.1-1ms（取决于文本长度）
- 总开销：通常 < 10ms

### 内存使用

- 每个占位符映射：约 100-200 字节
- 1000 个映射：约 100-200 KB

### 优化建议

1. **限制自定义规则数量**：过多的正则规则会增加延迟
2. **使用白名单**：对频繁出现的已知安全值添加白名单
3. **定期清理**：重启服务会清空内存映射

## 安全考虑

### 安全特性

✅ **不存储原始值**：只在内存中临时保存，不写入磁盘
✅ **审计日志**：仅存储元数据（类型、数量、前缀）
✅ **默认阻止**：如果脱敏失败，默认拒绝请求
✅ **HMAC 保护**：占位符使用密钥哈希，不可预测

### 风险和限制

⚠️ **重启丢失映射**：服务重启后，之前的占位符无法恢复（需要重新发送请求）
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
1. 服务重启导致映射丢失
2. 响应来自上游缓存

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

- `GET /api/admin/privacy-masking/config` - 获取配置
- `PUT /api/admin/privacy-masking/config` - 更新配置
- `GET /api/admin/privacy-masking/logs` - 查询日志
- `GET /api/admin/privacy-masking/stats` - 获取统计

## 参考

### 灵感来源

本功能参考了 [yetone/magpie](https://github.com/yetone/magpie) 项目的隐私脱敏实现，感谢开源社区的贡献。

### 许可证

- Sub2API: LGPL-3.0
- Magpie: MIT

## 支持

如有问题或建议，请提交 Issue 或 Pull Request。
