# Phase 14.6 Acceptance Logs

由 `scripts/smoke/phase14-6-acceptance.ps1` 自动生成，文件名格式：`acceptance-YYYYMMDD-HHmmss.log`。

## 用途

- 完整记录每次准入跑的所有 stdout/stderr，便于事后排障与签字归档；
- 通过 PowerShell `Start-Transcript` 实现，包含每个步骤的耗时和最终 PASS/FAIL 摘要。

## 关闭方式

```powershell
powershell -ExecutionPolicy Bypass -File scripts/smoke/phase14-6-acceptance.ps1 -NoLogFile
```

## 自定义目录

```powershell
... -LogDir D:\some\other\dir
```

## Git 状态

`*.log` 已被根 `.gitignore` 忽略，**日志本身不入库**。本 README 仅说明结构。

需要长期归档时，请手动复制日志到对外归档位置（例如 `docs/launch/Phase14.7-三签字流程.md` 引用），并附加在签字记录中。
