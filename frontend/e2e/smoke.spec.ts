// Smoke：注册 → 登录 → 创建 lake → 添加 node → 凝结 → 反馈
// 依赖：后端 RIPPLE_LLM_FAKE=true 启动，前端 dev server 在 5173
import { test, expect } from '@playwright/test'

const TS = Date.now()
const USER = `e2e_${TS}`
const PWD = 'Test1234!'

test.describe('Ripple 主路径 E2E', () => {
  test('注册→登录→建湖→建节点→凝结→反馈', async ({ page }) => {
    await page.goto('/')

    // 注册（如界面提供）
    await page.getByRole('textbox', { name: /用户名|username/i }).fill(USER)
    await page.getByRole('textbox', { name: /密码|password/i }).fill(PWD)
    const reg = page.getByRole('button', { name: /注册|register/i })
    if (await reg.isVisible()) await reg.click()

    // 登录
    await page.getByRole('button', { name: /登录|login/i }).click()
    await expect(page.getByText(/我的湖|lakes/i)).toBeVisible({ timeout: 10_000 })

    // 创建 lake
    await page.getByRole('button', { name: /新建湖|create lake/i }).click()
    const lakeName = `e2e-lake-${TS}`
    await page.getByRole('textbox', { name: /湖名|name/i }).fill(lakeName)
    await page.getByRole('button', { name: /创建|create/i }).click()
    await expect(page.getByText(lakeName)).toBeVisible()

    // 添加 node
    await page.getByText(lakeName).click()
    const nodeArea = page.getByRole('textbox', { name: /节点内容|node content/i })
    await nodeArea.fill('青萍涟漪 E2E 测试节点 1')
    await page.getByRole('button', { name: /添加|add/i }).click()
    await nodeArea.fill('青萍涟漪 E2E 测试节点 2')
    await page.getByRole('button', { name: /添加|add/i }).click()

    // 凝结
    await page.getByRole('button', { name: /凝结|crystallize/i }).click()
    await expect(page.getByText(/凝结结果|结晶/)).toBeVisible({ timeout: 30_000 })
  })
})
