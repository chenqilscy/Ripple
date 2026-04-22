// Playwright E2E 配置 — Ripple Phase 5 T6
// 运行前置：
//   1. cd frontend && npm i -D @playwright/test
//   2. npx playwright install chromium
//   3. 启动后端：cd backend-go && go run ./cmd/server
//   4. 启动前端：cd frontend && npm run dev
//   5. 跑测试：npx playwright test
import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './e2e',
  timeout: 30_000,
  retries: 0,
  reporter: [['list']],
  use: {
    baseURL: process.env.E2E_BASE_URL || 'http://localhost:5173',
    trace: 'on-first-retry',
  },
  projects: [
    { name: 'chromium', use: { browserName: 'chromium' } },
  ],
})
