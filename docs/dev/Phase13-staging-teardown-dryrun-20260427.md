# Phase 13 Staging Teardown Dry Run（2026-04-27）

## 目标

验证 `docs/ops/Phase13-联调与回滚手册.md` 中的非破坏回收演练步骤，确保非作者可在不停止容器、不删除卷的前提下确认回收命令。

## 执行命令

```powershell
Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass
./scripts/teardown-staging.ps1 -KeepVolumes -DryRun
```

## 结果

输出：

```text
DRY RUN: docker compose -f docker-compose.staging.yml down --remove-orphans
```

## 结论

- `-DryRun` 可在不依赖 Docker CLI 的情况下完成命令预览；
- `-KeepVolumes` 路径不会附加 `-v`，符合“非破坏演练”预期；
- 本轮未执行实际 `docker compose down`，未停止远端 staging，也未删除任何卷；
- 后续完整 staging 回收实操已在独立窗口执行并通过，见 `docs/dev/Phase13-staging-teardown-20260427.md`。
