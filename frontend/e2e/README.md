# Playwright E2E 测试

## 一次性安装

```powershell
cd frontend
npm i -D @playwright/test
npm run e2e:install   # 仅安装 chromium，避免下载所有浏览器
```

## 运行

```powershell
# 终端 1：后端（fake LLM 加速凝结）
cd backend-go
$env:RIPPLE_LLM_FAKE = "true"
$env:RIPPLE_LLM_FAKE_SLEEP_MS = "50"
go run ./cmd/server

# 终端 2：前端
cd frontend
npm run dev

# 终端 3：跑测试
cd frontend
npm run e2e
```

## 用例

- `e2e/smoke.spec.ts`：注册 → 登录 → 创建 lake → 添加 2 个 node → 触发凝结 → 验证晶体出现

## 已知限制（Spike 阶段）

- 选择器使用宽松正则匹配（中英文双语），如界面文案变更需同步更新。
- 未引入 dataTestId 体系；正式回归前应给关键控件加 `data-testid="..."`.
- 反馈/推荐路径未覆盖（待 P6）。
