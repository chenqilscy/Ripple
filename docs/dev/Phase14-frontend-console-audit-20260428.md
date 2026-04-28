# Frontend Console 输出审计

- **审计时间**：2026-04-28
- **审计范围**：`frontend/src/**/*.{ts,tsx}`
- **审计标准**：禁止 `console.log` / `console.debug` / `console.info` 进入主分支；允许异常路径的 `console.warn` / `console.error`。

## 审计结果

| 类别 | 命中数 | 处置 |
|------|-------|------|
| `console.log` | 0 | — |
| `console.debug` | 0 | — |
| `console.info` | 0 | — |
| `console.warn` | 2（CollabDemo.tsx:64, 76）| 保留：均位于 `.catch` 异常上报路径，符合规范 |
| `console.error` | 0 | — |

## 结论

✅ 当前代码无任何调试用 console 残留，符合 Phase 14 准入。

## 防回归建议

在 `frontend/.eslintrc` 中加入规则：

```json
"no-console": ["warn", { "allow": ["warn", "error"] }]
```

待 Phase 15 引入。
