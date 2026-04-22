// Smoke：注册→登录→创建湖→看见湖（P6-E 真实跑通版本）
// 依赖：后端 :8000 已启动；前端 dev :5173 已启动。
import { test, expect } from '@playwright/test'

const TS = Date.now()
const EMAIL = `e2e_${TS}@ripple.test`
const PWD = 'Test12345!'

test.describe('Ripple 主路径 E2E', () => {
  test('注册→登录→建湖→见湖', async ({ page }) => {
    await page.goto('/')

    // Login.tsx 默认 mode='login'，先切到 register
    const switchBtn = page.getByRole('button', { name: /还没账号？注册/ })
    if (await switchBtn.isVisible().catch(() => false)) await switchBtn.click()

    await page.getByPlaceholder('邮箱').fill(EMAIL)
    await page.getByPlaceholder(/密码（≥8 位）/).fill(PWD)
    const nick = page.getByPlaceholder(/昵称（可选）/)
    if (await nick.isVisible().catch(() => false)) await nick.fill('e2e')

    await page.getByRole('button', { name: /注册并入湖/ }).click()

    // 进入 Home，校验“青萍 · 我的湖”
    await expect(page.getByText('青萍 · 我的湖')).toBeVisible({ timeout: 15_000 })

    // 创建湖
    const lakeName = `e2e-lake-${TS}`
    await page.getByPlaceholder('新湖名…').fill(lakeName)
    await page.getByRole('button', { name: '+' }).nth(1).click()

    // 列表中出现湖名（既出现在侧栏 list 也出现在主区 heading；取第一个即可）
    await expect(page.getByText(lakeName).first()).toBeVisible({ timeout: 10_000 })
  })
})
