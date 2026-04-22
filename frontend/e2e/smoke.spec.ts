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

  test('建湖后 CollabDemo 渲染（P7-E）', async ({ page }) => {
    const email2 = `e2e_collab_${TS}@ripple.test`
    await page.goto('/')

    const switchBtn = page.getByRole('button', { name: /还没账号？注册/ })
    if (await switchBtn.isVisible().catch(() => false)) await switchBtn.click()

    await page.getByPlaceholder('邮箱').fill(email2)
    await page.getByPlaceholder(/密码（≥8 位）/).fill(PWD)
    const nick = page.getByPlaceholder(/昵称（可选）/)
    if (await nick.isVisible().catch(() => false)) await nick.fill('e2e-collab')

    await page.getByRole('button', { name: /注册并入湖/ }).click()
    await expect(page.getByText('青萍 · 我的湖')).toBeVisible({ timeout: 15_000 })

    // 建湖
    const collabLake = `collab-${TS}`
    await page.getByPlaceholder('新湖名…').fill(collabLake)
    await page.getByRole('button', { name: '+' }).nth(1).click()
    await expect(page.getByText(collabLake).first()).toBeVisible({ timeout: 10_000 })

    // 点击湖名使其 active（auto-select 或手动点击均可触发 CollabDemo 渲染）
    await page.getByText(collabLake).first().click()

    // P7-E：验证 CollabDemo 渲染（含"协作 demo"字样，不验 WS 连通性）
    await expect(page.getByText(/协作 demo/)).toBeVisible({ timeout: 8_000 })
  })
})
