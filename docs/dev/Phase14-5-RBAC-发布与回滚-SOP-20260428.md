# Phase 14.5 RBAC 发布与回滚 SOP

日期：2026-04-28  
文档层级：`docs/dev/*.md`，承接 `docs/dev/Phase14-5-RBAC-设计草案-20260428.md`。

## 1. 发布内容

- 后端新增 `platform_admins` 数据表与仓库；
- 后端新增平台管理员集中鉴权 helper；
- 后端新增 OWNER 管理 API：
  - `GET /api/v1/admin/platform_admins`
  - `POST /api/v1/admin/platform_admins`
  - `DELETE /api/v1/admin/platform_admins/{user_id}`
- 后端继续兼容 `RIPPLE_ADMIN_EMAILS`，命中邮箱白名单的用户视为 bootstrap OWNER；
- 平台管理员运营端点拒绝 API Key 继承权限，仅接受 JWT 用户会话；
- 前端 Settings 新增“平台管理员 RBAC”面板，可按用户 ID 或邮箱授权、撤销、查看列表。

## 2. 本地验证

```text
go test -race -count=1 ./...
go vet ./...
npm.cmd run lint
npm.cmd run build
```

已验证：

- `go test -race -count=1 ./...` 通过；
- `go vet ./...` 通过；
- `npm.cmd run lint` 通过；
- `npm.cmd run build` 通过。

## 3. Staging 发布步骤

> 远端 `/home/admin/Ripple` 为解包式部署目录，不是 git repo。

1. 本地打包：

```powershell
git archive --format=tar --output=$env:TEMP\ripple-backend-rbac.tar HEAD backend-go docs docker-compose.yml
```

2. 上传：

```powershell
scp $env:TEMP\ripple-backend-rbac.tar admin@fn.cky:/home/admin/ripple-backend-rbac.tar
```

3. 远端解包并重建后端：

```bash
cd /home/admin/Ripple
tar -xf /home/admin/ripple-backend-rbac.tar
docker compose -f docker-compose.staging.yml up -d --build migrate backend
```

4. 健康检查：

```powershell
Invoke-RestMethod -Uri 'http://fn.cky:18000/healthz' -Method Get
```

期望返回：`{"status":"ok"}`。

## 4. Staging smoke

使用浏览器内已有平台管理员 JWT 调用 staging API：

1. `POST /api/v1/admin/platform_admins` 给当前用户授予 `ADMIN`；
2. `GET /api/v1/admin/platform_admins` 返回包含当前用户；
3. `GET /api/v1/audit_logs?resource_type=platform_admin&resource_id=<user_id>` 返回 `platform_admin.grant`；
4. `DELETE /api/v1/admin/platform_admins/<user_id>` 返回 `204`；
5. 再查审计，包含 `platform_admin.grant` 与 `platform_admin.revoke`。

本轮结果：全部通过。

## 5. 回滚方案

### 5.1 应用回滚

若后端启动失败或 RBAC API 异常：

1. 回滚到上一稳定提交构建包；
2. 重新执行 staging 解包与 `docker compose -f docker-compose.staging.yml up -d --build migrate backend`；
3. 验证 `/healthz` 与原有 `/admin/overview`、`/admin/graylist`。

### 5.2 数据库回滚

若必须撤销数据表：

1. 先确认不再运行依赖 `platform_admins` 的后端版本；
2. 执行 `0020_platform_admins.down.sql`；
3. 保留 `RIPPLE_ADMIN_EMAILS` 作为平台管理员 bootstrap 兜底。

### 5.3 权限回滚

- 若误授予数据库平台管理员：优先调用 `DELETE /api/v1/admin/platform_admins/{user_id}` 软撤销；
- 若误配置 `RIPPLE_ADMIN_EMAILS`：修改环境变量并重启 backend；
- 注意：环境变量白名单优先于数据库授权，数据库撤销不能取消 env bootstrap OWNER。

## 6. 后续项

- 前端 RBAC 面板加入更明确的“当前账号角色”提示；
- 管理 API 增加分页与搜索；
- 将 graylist 写入与 audit log 写入收敛到事务化 service。
