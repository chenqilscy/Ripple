import { expect, test } from '@playwright/test'

async function mockRippleApi(page: import('@playwright/test').Page) {
  await page.route('**/api/v1/**', route => {
    const url = new URL(route.request().url())
    let body: unknown = {}
    if (url.pathname === '/api/v1/auth/login') {
      body = {
        access_token: 'e2e-token',
        token_type: 'bearer',
        user: { id: 'u-e2e', email: 'owner@example.com', display_name: 'Owner' },
      }
    } else if (url.pathname === '/api/v1/auth/me') {
      body = { id: 'u-e2e', email: 'owner@example.com', display_name: 'Owner' }
    } else if (url.pathname === '/api/v1/lakes') {
      body = { lakes: [] }
    } else if (url.pathname === '/api/v1/spaces') {
      body = { spaces: [] }
    } else if (url.pathname === '/api/v1/recommendations') {
      body = { recommendations: [] }
    } else if (url.pathname === '/api/v1/admin/overview') {
      body = {
        stats: { users_count: 1, organizations_count: 0, lakes_count: 0, graylist_entries_count: 0 },
        recent_organizations: [],
      }
    } else if (url.pathname === '/api/v1/admin/platform_admins') {
      body = { admins: [] }
    } else if (url.pathname === '/api/v1/api_keys') {
      body = { api_keys: [] }
    } else if (url.pathname === '/api/v1/organizations') {
      body = { organizations: [] }
    } else if (url.pathname === '/api/v1/admin/graylist') {
      body = { entries: [] }
    } else if (url.pathname === '/api/v1/audit_logs') {
      body = { events: [] }
    }
    return route.fulfill({ contentType: 'application/json', body: JSON.stringify(body) })
  })
}

test.describe('Settings 子 Tab', () => {
  test('可以在管理概览、平台管理员、API Key、灰度名单和审计日志之间切换', async ({ page }) => {
    await mockRippleApi(page)

    await page.goto('/')
    await page.getByPlaceholder('邮箱').fill('owner@example.com')
    await page.getByPlaceholder(/密码/).fill('Test12345!')
    await page.getByRole('button', { name: '入湖' }).click()
    await expect(page.getByText('青萍 · 我的湖')).toBeVisible()
    await page.getByRole('button', { name: '⚙' }).click()

    await expect(page.getByRole('heading', { name: '管理员总览' })).toBeVisible()

    await page.getByRole('button', { name: '平台管理员' }).click()
    await expect(page.getByRole('heading', { name: '平台管理员 RBAC' })).toBeVisible()

    await page.getByRole('button', { name: 'API Key' }).click()
    await expect(page.getByRole('heading', { name: 'API Key 管理' })).toBeVisible()

    await page.getByRole('button', { name: '灰度名单' }).click()
    await expect(page.getByRole('heading', { name: '灰度名单' })).toBeVisible()

    await page.getByRole('button', { name: '审计日志' }).click()
    await expect(page.getByRole('heading', { name: '审计日志' })).toBeVisible()
  })
})
