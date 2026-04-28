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
- 前端 `.tsx` 与 `.js` mirror 必须同步更新；发布前以 `npm.cmd run build` 验证实际产物包含 `PlatformAdminManager-*` chunk。
- 前端 nginx 必须代理 `/yjs` 到 `yjs-bridge:7790`，否则协作 WebSocket 会被 SPA fallback 返回 200。
- 后端 LakeWS 与 yjs-bridge 使用 `nhooyr.io/websocket`，其 `OriginPatterns` 只接受 host pattern（如 `fn.cky:14173`），不能直接传入完整 CORS URL。

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

### 3.1 后端发布

1. 本地打包后端与文档：

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

### 3.2 前端发布

1. 本地构建：

```powershell
cd frontend
npm.cmd run build
tar -cf $env:TEMP\ripple-frontend-rbac-ui.tar dist
```

2. 上传并热更新 nginx 容器：

```powershell
scp $env:TEMP\ripple-frontend-rbac-ui.tar admin@fn.cky:/home/admin/ripple-frontend-rbac-ui.tar
ssh admin@fn.cky "rm -rf /home/admin/ripple-dist-upload && mkdir -p /home/admin/ripple-dist-upload && tar -xf /home/admin/ripple-frontend-rbac-ui.tar -C /home/admin/ripple-dist-upload && docker cp /home/admin/ripple-dist-upload/dist/. ripple-staging-frontend:/usr/share/nginx/html/"
```

3. 浏览器刷新 `http://fn.cky:14173/`，Settings 页应出现“平台管理员 RBAC”面板。

### 3.3 前端 nginx / WebSocket 配置固化

当修改 [frontend/nginx.conf](../../frontend/nginx.conf) 时，仅热拷贝 `dist` 不会更新容器内 nginx 配置，必须执行其一：

1. 推荐：重建 frontend 镜像并重启容器：

```bash
cd /home/admin/Ripple
docker compose -f docker-compose.staging.yml up -d --build frontend yjs-bridge
```

2. 若 Docker Hub 元数据拉取卡顿，可临时热更新配置并 reload：

```bash
cd /home/admin/Ripple
docker compose -f docker-compose.staging.yml up -d --no-build yjs-bridge
docker cp frontend/nginx.conf ripple-staging-frontend:/etc/nginx/conf.d/default.conf
docker exec ripple-staging-frontend nginx -s reload
```

热更新只是 staging 应急措施；后续重建容器前必须确认源码中的 [frontend/nginx.conf](../../frontend/nginx.conf) 已包含 `/yjs` 代理段，否则配置会丢失。

### 3.4 WebSocket smoke

1. `/yjs` 不应再返回前端 HTML。无参数普通 HTTP smoke 期望来自 yjs-bridge 的 `400 lake required`：

```powershell
curl.exe -i --max-time 5 http://fn.cky:14173/yjs
```

2. LakeWS 浏览器 Origin smoke：对操作者有权限的湖发起 WebSocket Upgrade，请求头 `Origin: http://fn.cky:14173`，期望 `101 Switching Protocols`，不应再是 `403`。
3. yjs-bridge smoke：使用 `POST /api/v1/ws_token` 签发 ws-only token 后访问 `/yjs/<room>?lake=<lake_id>&node=<node_id>&token=<ws_token>`，期望 `101 Switching Protocols`。

## 4. Staging smoke

使用浏览器内已有平台 OWNER JWT 调用 staging API。操作者必须是 `RIPPLE_ADMIN_EMAILS` bootstrap OWNER 或数据库 `OWNER`；不要使用普通 `ADMIN` 账号执行授权操作。

建议使用独立测试用户作为授权目标；若临时使用当前 bootstrap OWNER 账号作为目标，必须在 smoke 结束后撤销数据库记录，保留 env bootstrap 兜底。

1. `POST /api/v1/admin/platform_admins` 给目标用户授予 `ADMIN`；
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
2. 导出 `platform_admins` 全表与相关审计记录（`resource_type=platform_admin`），由负责人确认备份可读；
3. 执行 `0020_platform_admins.down.sql`；
4. 保留 `RIPPLE_ADMIN_EMAILS` 作为平台管理员 bootstrap 兜底。

### 5.3 权限回滚

- 若误授予数据库平台管理员：优先调用 `DELETE /api/v1/admin/platform_admins/{user_id}` 软撤销；
- 若误配置 `RIPPLE_ADMIN_EMAILS`：修改环境变量并重启 backend；
- 注意：环境变量白名单优先于数据库授权，数据库撤销不能取消 env bootstrap OWNER。

## 6. 后续项

- 前端 RBAC 面板加入更明确的“当前账号角色”提示；
- 管理 API 增加分页与搜索；
- 将 graylist 写入与 audit log 写入收敛到事务化 service。
