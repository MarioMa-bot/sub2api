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

原始值到占位符的映射**仅存储在内存中**，不会写入磁盘或数据库。这确保：
- 重启后映射丢失（下次请求会重新生成）
- 没有持久化的安全风险
- 性能优秀

### 安装和配置

#### 1. 数据库迁移

脱敏功能需要两个数据库表：
- `privacy_masking_config`：存储配置
- `privacy_masking_log`：存储审计日志

迁移文件已包含在 `backend/migrations/250_privacy_masking.sql`，运行应用时会自动应用。

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

确认 `250_privacy_masking.sql` 迁移已应用。

#### 5. 访问管理界面

打开浏览器访问：`https://your-domain.com/admin/privacy-masking`

### 监控和审计

#### 查看脱敏日志

在管理界面的 "最近的脱敏日志" 部分可以看到：
- 检测时间
- 检测到的类型和数量
- 采取的动作
- 请求是否被阻止

#### API 查询

```bash
# 获取所有日志
GET /api/admin/privacy-masking/logs

# 过滤参数
GET /api/admin/privacy-masking/logs?user_id=123&blocked=true&limit=100
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

Original value ↔ placeholder mappings are stored **in memory only**, never written to disk or database. This ensures:
- Mappings are lost on restart (regenerated on next request)
- No persistent security risks
- Excellent performance

### Installation and Configuration

#### 1. Database Migration

The masking feature requires two database tables:
- `privacy_masking_config`: Store configuration
- `privacy_masking_log`: Store audit logs

Migration file is included at `backend/migrations/250_privacy_masking.sql` and will be applied automatically.

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

In the "Recent Masking Logs" section, you can see:
- Detection time
- Types and counts detected
- Actions taken
- Whether request was blocked

#### API Queries

```bash
# Get all logs
GET /api/admin/privacy-masking/logs

# With filters
GET /api/admin/privacy-masking/logs?user_id=123&blocked=true&limit=100
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
