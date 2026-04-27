# Phase 13 联调与回滚手册

> 目标：让未参与开发的人也能独立完成 staging 拉起、冒烟、故障演练、环境回收。

---

## 1. 前置条件

1. 已安装 Docker Desktop 或等价 Docker Engine。
2. PowerShell 可执行本地脚本：

```powershell
Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass
```

3. 当前 PowerShell 会话已注入下列变量：

```powershell
$env:PG_PASSWORD="replace-me"
$env:NEO4J_PASSWORD="replace-me"
$env:REDIS_PASSWORD="replace-me"
$env:JWT_SECRET="replace-me-with-at-least-32-chars"
```

---

## 2. 拉起 staging

```powershell
./scripts/bootstrap-staging.ps1
```

默认地址：

- 前端：`http://127.0.0.1:14173`
- 后端：`http://127.0.0.1:18000`
- Yjs：`ws://127.0.0.1:17790/yjs`

若只想拉起环境，不执行冒烟：

```powershell
./scripts/bootstrap-staging.ps1 -SkipSmoke
```

---

## 3. 手工检查项

1. 打开前端首页，确认能进入登录页。
2. 访问 `http://127.0.0.1:18000/healthz`，确认返回 `{"status":"ok"}`。
3. 访问 `http://127.0.0.1:18000/metrics`，确认能看到 Prometheus 文本。
4. 执行：

```powershell
./scripts/smoke/phase13-smoke.ps1 -Base http://127.0.0.1:18000
```

若要执行第一次完整联调，请按 [../dev/Phase13-真实联调执行清单.md](../dev/Phase13-真实联调执行清单.md) 逐项回填。

执行完成后，再按 [../dev/Phase13-联调结果回填模板.md](../dev/Phase13-联调结果回填模板.md) 把结果写回任务清单与准入清单。

---

## 4. 故障演练

支持的场景：`redis`、`neo4j`、`yjs-bridge`。

```powershell
# 注入故障（默认 15 秒后自动恢复）
./scripts/drill-staging.ps1 -Scenario redis -DurationSeconds 15

# 只停止，不自动恢复
./scripts/drill-staging.ps1 -Scenario neo4j -NoRecover
```

观察点：

1. `http://127.0.0.1:18000/healthz` 是否仍能恢复为 `ok`。
2. 搜索、批量导入、协作相关功能是否在恢复后重新可用。
3. 后端日志中是否存在持续性错误而非瞬时错误。

---

## 5. 回滚 / 回收

非破坏演练（只打印将执行的命令，不停容器、不删卷）：

```powershell
./scripts/teardown-staging.ps1 -KeepVolumes -DryRun
```

> 非作者首次复现时先跑 `-DryRun`；确认 compose 文件、路径与参数无误后，再进入实际回收窗口。

完整回收（删除卷）：

```powershell
./scripts/teardown-staging.ps1
```

保留卷，仅停容器：

```powershell
./scripts/teardown-staging.ps1 -KeepVolumes
```

应用级回滚建议：

1. 记录当前灰度镜像 tag / commit。
2. 回退到上一稳定 tag。
3. 若涉及 schema 变更，先在独立窗口验证 down 迁移。
4. 回滚后重新执行 `phase13-smoke.ps1` 确认恢复。

若要在 staging 做一次 Git ref 级别回滚演练：

```powershell
./scripts/rollback-staging.ps1 -Ref <stable-ref>
```

该脚本默认要求干净工作树，并在切换目标 ref 前自动执行 teardown，再重新 bootstrap。

---

## 6. 常见问题

| 现象 | 排查 |
|------|------|
| `docker` 命令不存在 | 当前机器未安装 Docker；改到联调机执行 |
| `healthz` 一直不绿 | 用 `docker compose -f docker-compose.staging.yml logs backend` 检查启动错误 |
| 冒烟卡在建湖后搜索 | 检查 Neo4j 是否健康、outbox 是否已处理 |
| 故障演练后未恢复 | 先执行 `teardown-staging.ps1` 再 `bootstrap-staging.ps1 -SkipSmoke` |