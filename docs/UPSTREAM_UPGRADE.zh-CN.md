# 上游升级指南 | Upstream Upgrade Guide

本文档说明如何从上游仓库 [Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api) 拉取最新更新，同时保留本 fork 中添加的隐私脱敏功能。

This document explains how to pull upstream updates from [Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api) while preserving the privacy masking feature added in this fork.

---

## 目录 | Table of Contents

- [准备工作](#准备工作)
- [配置上游远程仓库](#配置上游远程仓库)
- [拉取上游更新](#拉取上游更新)
- [解决冲突](#解决冲突)
- [常见冲突位置](#常见冲突位置)
- [测试和验证](#测试和验证)
- [部署更新](#部署更新)

---

## 准备工作

在开始之前，确保：

1. **备份数据库**：升级前务必备份生产数据库
2. **测试环境验证**：先在测试环境执行升级流程
3. **了解变更**：查看上游的 CHANGELOG 或 Release Notes

```bash
# 备份数据库（PostgreSQL 示例）
pg_dump -U postgres -d sub2api > backup_$(date +%Y%m%d_%H%M%S).sql

# 或使用 Docker
docker-compose exec postgres pg_dump -U postgres sub2api > backup_$(date +%Y%m%d_%H%M%S).sql
```

---

## 配置上游远程仓库

### 第一次配置（只需执行一次）

```bash
cd /path/to/sub2api

# 添加上游远程仓库
git remote add upstream https://github.com/Wei-Shaw/sub2api.git

# 验证远程仓库配置
git remote -v
# 应该看到：
# origin    https://github.com/MarioMa-bot/sub2api.git (fetch)
# origin    https://github.com/MarioMa-bot/sub2api.git (push)
# upstream  https://github.com/Wei-Shaw/sub2api.git (fetch)
# upstream  https://github.com/Wei-Shaw/sub2api.git (push)
```

### 检查现有配置

```bash
# 如果之前已配置，检查是否正确
git remote -v

# 如果配置不正确，可以删除后重新添加
git remote remove upstream
git remote add upstream https://github.com/Wei-Shaw/sub2api.git
```

---

## 拉取上游更新

### 方法一：合并（Merge）- 推荐用于生产环境

合并会保留完整的提交历史，适合生产环境。

```bash
# 1. 确保在 main 分支
git checkout main

# 2. 拉取本地最新代码
git pull origin main

# 3. 获取上游更新
git fetch upstream

# 4. 查看上游有哪些新提交
git log main..upstream/main --oneline

# 5. 合并上游 main 分支
git merge upstream/main

# 如果有冲突，解决后继续（见下文"解决冲突"部分）
```

### 方法二：变基（Rebase）- 适合开发环境

变基会生成更干净的线性历史，但不推荐用于已推送的生产分支。

```bash
# 1. 确保在 main 分支
git checkout main

# 2. 获取上游更新
git fetch upstream

# 3. 变基到上游 main
git rebase upstream/main

# 如果有冲突，解决后继续
git rebase --continue

# 完成后强制推送（谨慎使用！）
git push origin main --force-with-lease
```

**⚠️ 警告**：`rebase` 会改写历史，如果团队其他成员也在使用这个分支，请使用 `merge` 方式。

---

## 解决冲突

### 冲突发生时的处理流程

```bash
# 1. 查看冲突文件
git status

# 2. 打开冲突文件，手动解决冲突
# 冲突标记格式：
# <<<<<<< HEAD
# 你的更改
# =======
# 上游的更改
# >>>>>>> upstream/main

# 3. 解决冲突后，标记为已解决
git add <冲突文件>

# 4. 继续合并
git merge --continue

# 或者如果是 rebase
git rebase --continue
```

### 放弃合并（如果需要重新开始）

```bash
# 放弃合并
git merge --abort

# 或放弃 rebase
git rebase --abort
```

---

## 常见冲突位置

根据隐私脱敏功能的实现，以下位置可能会发生冲突：

### 1. 数据库迁移编号冲突

**文件位置**：`backend/migrations/250_privacy_masking.sql`

**冲突原因**：上游可能添加了 250 号或更高编号的迁移。

**解决方案**：

```bash
# 1. 查看上游最新的迁移编号
ls -la backend/migrations/*.sql | tail -5

# 2. 重命名我们的迁移文件为更高的编号
# 假设上游最新是 255，我们改为 256
git mv backend/migrations/250_privacy_masking.sql backend/migrations/256_privacy_masking.sql

# 3. 标记为已解决
git add backend/migrations/
git merge --continue
```

### 2. 前端路由冲突

**文件位置**：`frontend/src/router/index.ts`

**冲突原因**：上游可能添加了新的管理路由。

**解决方案**：

```typescript
// 保留双方的路由，合并到一起
{
  path: '/admin',
  children: [
    // ... 上游的路由
    {
      path: 'privacy-masking',  // 我们的路由
      name: 'AdminPrivacyMasking',
      component: () => import('@/views/admin/PrivacyMaskingView.vue'),
    },
  ]
}
```

### 3. i18n 索引文件冲突

**文件位置**：
- `frontend/src/i18n/locales/zh/admin/index.ts`
- `frontend/src/i18n/locales/en/admin/index.ts`

**冲突原因**：上游可能添加了新的翻译模块。

**解决方案**：

```typescript
// 保留双方的导入，按字母顺序排列
import accounts from './accounts'
import audit from './audit'
import channels from './channels'
import ops from './ops'
import overview from './overview'
import plugins from './plugins'
import privacyMasking from './privacyMasking'  // 我们的
import promptAudit from './promptAudit'
import resources from './resources'
import settings from './settings'
// ... 上游可能添加的新模块

export default {
  ...accounts,
  ...audit,
  ...channels,
  ...ops,
  ...overview,
  ...plugins,
  ...privacyMasking,  // 我们的
  ...promptAudit,
  ...resources,
  ...settings,
  // ... 上游可能添加的新模块
}
```

### 4. 后端服务初始化冲突

**文件位置**：`backend/cmd/server/main.go` 或类似的启动文件

**冲突原因**：服务初始化代码可能被修改。

**解决方案**：

```go
// 保留上游的修改，同时添加我们的服务初始化
func initServices(db *gorm.DB) {
    // ... 上游的服务初始化

    // 隐私脱敏服务（我们添加的）
    maskingService, _ := service.NewPrivacyMaskingService()
    maskingRepo := repository.NewPrivacyMaskingRepository(db)
    maskingHandler := admin.NewPrivacyMaskingHandler(maskingRepo, logger)
    
    // 注册路由
    adminGroup.GET("/privacy-masking/config", maskingHandler.GetGlobalConfig)
    adminGroup.PUT("/privacy-masking/config", maskingHandler.UpdateGlobalConfig)
    // ...
}
```

### 5. 前端管理菜单冲突

**文件位置**：`frontend/src/components/AdminSidebar.vue` 或类似文件

**冲突原因**：上游可能修改了管理菜单结构。

**解决方案**：

```vue
<template>
  <!-- 保留上游的菜单项 -->
  <MenuItem to="/admin/dashboard">控制台</MenuItem>
  <MenuItem to="/admin/users">用户管理</MenuItem>
  <!-- ... -->
  
  <!-- 添加我们的菜单项 -->
  <MenuItem to="/admin/privacy-masking">隐私脱敏</MenuItem>
</template>
```

---

## 测试和验证

升级完成后，务必进行全面测试：

### 1. 编译检查

```bash
# 后端编译
cd backend
go build -v ./...

# 前端编译
cd frontend
npm run build
```

### 2. 运行测试

```bash
# 后端测试
cd backend
go test ./... -v

# 隐私脱敏模块测试
go test -v ./internal/service -run TestPrivacyMasking

# 前端测试
cd frontend
npm run test
```

### 3. 本地运行验证

```bash
# 使用 Docker Compose 本地运行
docker-compose down
docker-compose build
docker-compose up -d

# 检查服务状态
docker-compose ps
docker-compose logs backend | grep -i "migration\|privacy"

# 访问管理界面
# http://localhost:3000/admin/privacy-masking
```

### 4. 功能验证清单

- [ ] 后端服务正常启动
- [ ] 数据库迁移成功执行
- [ ] 隐私脱敏配置页面可访问
- [ ] 可以正常保存脱敏配置
- [ ] 发送测试请求，验证脱敏功能工作
- [ ] 查看脱敏日志正常显示
- [ ] 所有原有功能正常工作

---

## 部署更新

### 生产环境部署流程

```bash
# 1. 停止服务
docker-compose down

# 2. 备份数据库（重要！）
docker-compose exec postgres pg_dump -U postgres sub2api > backup_before_upgrade.sql

# 3. 拉取最新代码
git pull origin main

# 4. 重新构建镜像
docker-compose build

# 5. 启动服务
docker-compose up -d

# 6. 检查迁移日志
docker-compose logs backend | grep migration

# 7. 验证服务健康状态
curl http://localhost:8080/health
# 或
docker-compose exec backend curl http://localhost:8080/health

# 8. 检查隐私脱敏功能
# 访问管理界面并测试
```

### 回滚流程（如果升级失败）

```bash
# 1. 停止服务
docker-compose down

# 2. 回滚代码
git reset --hard <上次工作的commit>
# 或
git revert <出问题的commit>

# 3. 恢复数据库（如果需要）
docker-compose up -d postgres
docker-compose exec -T postgres psql -U postgres sub2api < backup_before_upgrade.sql

# 4. 重新构建并启动
docker-compose build
docker-compose up -d

# 5. 验证服务恢复正常
```

---

## 自动化升级脚本

可以创建一个脚本来自动化升级流程：

```bash
#!/bin/bash
# upgrade-from-upstream.sh

set -e  # 遇到错误立即退出

echo "=== Sub2API 上游升级脚本 ==="

# 1. 检查当前分支
CURRENT_BRANCH=$(git branch --show-current)
if [ "$CURRENT_BRANCH" != "main" ]; then
    echo "错误：请在 main 分支上执行此脚本"
    exit 1
fi

# 2. 检查是否有未提交的更改
if ! git diff-index --quiet HEAD --; then
    echo "错误：存在未提交的更改，请先提交或暂存"
    exit 1
fi

# 3. 备份数据库
echo "正在备份数据库..."
BACKUP_FILE="backup_$(date +%Y%m%d_%H%M%S).sql"
docker-compose exec -T postgres pg_dump -U postgres sub2api > "$BACKUP_FILE"
echo "数据库已备份到: $BACKUP_FILE"

# 4. 拉取本地最新代码
echo "拉取本地最新代码..."
git pull origin main

# 5. 获取上游更新
echo "获取上游更新..."
git fetch upstream

# 6. 显示上游新提交
echo "=== 上游新提交 ==="
git log main..upstream/main --oneline --graph --decorate

# 7. 询问是否继续
read -p "是否继续合并上游更新？(y/n) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "已取消"
    exit 0
fi

# 8. 合并上游
echo "合并上游 main 分支..."
if git merge upstream/main --no-edit; then
    echo "合并成功！"
else
    echo "发生冲突，请手动解决后运行："
    echo "  git merge --continue"
    echo "  ./deploy-upgrade.sh"
    exit 1
fi

# 9. 推送到远程
echo "推送到远程仓库..."
git push origin main

echo "=== 升级完成！==="
echo "请运行 ./deploy-upgrade.sh 部署到生产环境"
```

部署脚本 `deploy-upgrade.sh`：

```bash
#!/bin/bash
# deploy-upgrade.sh

set -e

echo "=== 部署升级到生产环境 ==="

# 1. 停止服务
echo "停止服务..."
docker-compose down

# 2. 重新构建
echo "重新构建镜像..."
docker-compose build

# 3. 启动服务
echo "启动服务..."
docker-compose up -d

# 4. 等待服务启动
echo "等待服务启动..."
sleep 10

# 5. 检查服务状态
echo "检查服务状态..."
docker-compose ps

# 6. 检查迁移日志
echo "=== 迁移日志 ==="
docker-compose logs backend | grep migration | tail -20

# 7. 健康检查
echo "=== 健康检查 ==="
if curl -f http://localhost:8080/health > /dev/null 2>&1; then
    echo "✓ 后端服务健康"
else
    echo "✗ 后端服务异常"
    exit 1
fi

echo "=== 部署完成！==="
echo "请访问管理界面验证功能是否正常"
```

使用方法：

```bash
# 1. 赋予执行权限
chmod +x upgrade-from-upstream.sh
chmod +x deploy-upgrade.sh

# 2. 执行升级
./upgrade-from-upstream.sh

# 3. 如果升级成功，部署到生产
./deploy-upgrade.sh
```

---

## 注意事项

### 升级频率建议

- **安全更新**：立即升级
- **功能更新**：每月或每季度升级
- **重大版本**：充分测试后再升级

### 维护窗口

建议在业务低峰期进行升级，预留足够的时间进行测试和回滚。

### 版本兼容性

注意检查：
- Go 版本兼容性
- Node.js 版本兼容性
- 数据库 schema 变更
- API 接口变更

---

## 获取帮助

如果在升级过程中遇到问题：

1. **查看上游文档**：检查 Wei-Shaw/sub2api 的 CHANGELOG 和 Wiki
2. **搜索 Issues**：在上游仓库搜索类似问题
3. **提交 Issue**：在本 fork 或上游仓库提交问题
4. **社区支持**：在相关社区论坛寻求帮助

---

## 相关文档

- [隐私脱敏功能文档](../PRIVACY_MASKING.zh-CN.md)
- [部署文档](../docs/DEPLOYMENT.md)
- [上游项目](https://github.com/Wei-Shaw/sub2api)

---

**最后更新**：2026-11-04  
**维护者**：MarioMa-bot
