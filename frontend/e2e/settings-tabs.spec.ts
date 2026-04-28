import { expect, test } from '@playwright/test'

interface MockState {
  graylist: { id: string; email: string; note: string; created_by: string; created_at: string }[]
}

function makeMock(state: MockState) {
  return async function mockRippleApi(page: import('@playwright/test').Page) {
    await page.route('**/api/v1/**', route => {
      const url = new URL(route.request().url())
      const method = route.request().method()
      let body: unknown = {}
      let status = 200

      if (url.pathname === '/api/v1/auth/login' && method === 'POST') {
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
          stats: {
            users_count: 1,
            organizations_count: 0,
            lakes_count: 0,
            graylist_entries_count: state.graylist.length,
          },
          recent_organizations: [],
        }
      } else if (url.pathname === '/api/v1/admin/platform_admins') {
        body = { admins: [] }
      } else if (url.pathname === '/api/v1/api_keys') {
        body = { api_keys: [] }
      } else if (url.pathname === '/api/v1/organizations') {
        body = { organizations: [] }
      } else if (url.pathname === '/api/v1/admin/graylist' && method === 'GET') {
        body = { entries: state.graylist }
      } else if (url.pathname === '/api/v1/admin/graylist' && method === 'POST') {
        const payload = JSON.parse(route.request().postData() ?? '{}') as {
          email?: string
          note?: string
        }
        const entry = {
          id: `g-${state.graylist.length + 1}`,
          email: payload.email ?? '',
          note: payload.note ?? '',
          created_by: 'u-e2e',
          created_at: new Date().toISOString(),
        }
        state.graylist = [entry, ...state.graylist.filter(e => e.email !== entry.email)]
        body = entry
      } else if (url.pathname.startsWith('/api/v1/admin/graylist/') && method === 'DELETE') {
        const id = url.pathname.split('/').pop()
        state.graylist = state.graylist.filter(e => e.id !== id)
        status = 204
        body = {}
      } else if (url.pathname === '/api/v1/audit_logs') {
        body = { events: [] }
      }
      return route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) })
    })
  }
}

async function loginAndOpenSettings(page: import('@playwright/test').Page) {
  await page.goto('/')
  await page.getByPlaceholder('邮箱').fill('owner@example.com')
  await page.getByPlaceholder(/密码/).fill('Test12345!')
  await page.getByRole('button', { name: '入湖' }).click()
  await expect(page.getByText('青萍 · 我的湖')).toBeVisible()
  await page.getByRole('button', { name: '⚙' }).click()
}

test.describe('Settings 子 Tab', () => {
  test('可以在管理概览、平台管理员、API Key、灰度名单和审计日志之间切换', async ({ page }) => {
    const state: MockState = { graylist: [] }
    await makeMock(state)(page)
    await loginAndOpenSettings(page)

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

  test('灰度名单可以新增并删除一条记录', async ({ page }) => {
    const state: MockState = { graylist: [] }
    await makeMock(state)(page)
    await loginAndOpenSettings(page)

    await page.getByRole('button', { name: '灰度名单' }).click()
    await expect(page.getByRole('heading', { name: '灰度名单' })).toBeVisible()

    await page.getByPlaceholder('允许注册的邮箱').fill('phase14_smoke_e2e@ripple.test')
    await page.getByPlaceholder('备注（可选）').fill('e2e smoke')
    await page.getByRole('button', { name: /添加 \/ 更新/ }).click()

    await expect(page.getByText('phase14_smoke_e2e@ripple.test')).toBeVisible()
    expect(state.graylist).toHaveLength(1)
    expect(state.graylist[0].email).toBe('phase14_smoke_e2e@ripple.test')

    page.once('dialog', dialog => void dialog.accept())
    await page.getByRole('button', { name: '移除' }).click()

    await expect(page.getByText('phase14_smoke_e2e@ripple.test')).toHaveCount(0)
    expect(state.graylist).toHaveLength(0)
  })
})

