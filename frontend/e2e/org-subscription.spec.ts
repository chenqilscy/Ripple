import { expect, test } from '@playwright/test'

interface MockState {
  currentSubscription: {
    id: string
    org_id: string
    plan_id: string
    status: 'active' | 'expired' | 'cancelled'
    billing_cycle: 'monthly' | 'annual'
    started_at: string
    expires_at: string
    created_at: string
  } | null
}

function iso(daysFromNow = 0) {
  return new Date(Date.now() + daysFromNow * 24 * 60 * 60 * 1000).toISOString()
}

function makeMock(state: MockState) {
  return async function mockRippleApi(page: import('@playwright/test').Page) {
    await page.route('**/api/v1/**', route => {
      const url = new URL(route.request().url())
      const method = route.request().method()
      let body: unknown = {}
      let status = 200
      let handled = true

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
      } else if (url.pathname === '/api/v1/notifications/unread_count') {
        body = { count: 0 }
      } else if (url.pathname === '/api/v1/notifications' && method === 'GET') {
        body = { notifications: [] }
      } else if (url.pathname === '/api/v1/subscriptions/plans') {
        body = {
          plans: [
            {
              id: 'free',
              name_zh: '免费版',
              price_cny_monthly: 0,
              quotas: { max_members: 3, max_lakes: 50, max_nodes: 10000, max_storage_mb: 1024 },
            },
            {
              id: 'pro',
              name_zh: '专业版',
              price_cny_monthly: 29,
              quotas: { max_members: 20, max_lakes: 500, max_nodes: 100000, max_storage_mb: 10240 },
            },
            {
              id: 'team',
              name_zh: '团队版',
              price_cny_monthly: 99,
              quotas: { max_members: 100, max_lakes: 5000, max_nodes: 1000000, max_storage_mb: 102400 },
            },
          ],
        }
      } else if (url.pathname === '/api/v1/organizations/org-e2e/subscription' && method === 'GET') {
        body = { subscription: state.currentSubscription }
      } else if (url.pathname === '/api/v1/organizations/org-e2e/subscription' && method === 'POST') {
        const payload = JSON.parse(route.request().postData() ?? '{}') as { plan_id?: string; billing_cycle?: 'monthly' | 'annual' }
        state.currentSubscription = {
          id: 'sub-e2e',
          org_id: 'org-e2e',
          plan_id: payload.plan_id ?? 'pro',
          status: 'active',
          billing_cycle: payload.billing_cycle ?? 'monthly',
          started_at: iso(0),
          expires_at: iso(payload.billing_cycle === 'annual' ? 365 : 30),
          created_at: iso(0),
        }
        body = { subscription: state.currentSubscription }
      } else if (url.pathname === '/api/v1/organizations/org-e2e/usage') {
        body = { usage: { members: 2, lakes: 1, nodes: 42 } }
      } else if (url.pathname === '/api/v1/organizations/org-e2e/llm_usage') {
        body = {
          org_id: 'org-e2e',
          period_days: 30,
          total_calls: 12,
          total_estimated_cost_cny: 0.12,
          by_provider: [
            { provider: 'zhipu', calls: 10, avg_duration_ms: 1200, estimated_cost_cny: 0.1 },
            { provider: 'openai', calls: 2, avg_duration_ms: 1800, estimated_cost_cny: 0.02 },
          ],
          by_day: [
            { date: '2026-04-23', calls: 1, estimated_cost_cny: 0.01 },
            { date: '2026-04-24', calls: 2, estimated_cost_cny: 0.02 },
            { date: '2026-04-25', calls: 0, estimated_cost_cny: 0 },
            { date: '2026-04-26', calls: 3, estimated_cost_cny: 0.03 },
            { date: '2026-04-27', calls: 1, estimated_cost_cny: 0.01 },
            { date: '2026-04-28', calls: 2, estimated_cost_cny: 0.02 },
            { date: '2026-04-29', calls: 3, estimated_cost_cny: 0.03 },
          ],
        }
      } else {
        handled = false
      }

      if (!handled) {
        throw new Error(`Unhandled API request in org-subscription.spec.ts: ${method} ${url.pathname}`)
      }

      return route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) })
    })
  }
}

async function openHarness(page: import('@playwright/test').Page) {
  await page.addInitScript(() => window.localStorage.setItem('ripple.token', 'e2e-token'))
  await page.goto('/subscription-harness.html')
  await expect(page.getByRole('heading', { name: 'Subscription Harness' })).toBeVisible({ timeout: 10_000 })
}

test.describe('Organization subscription and billing', () => {
  test('可查看 AI 账单并切换到团队年付', async ({ page }) => {
    const state: MockState = {
      currentSubscription: {
        id: 'sub-e2e',
        org_id: 'org-e2e',
        plan_id: 'pro',
        status: 'active',
        billing_cycle: 'monthly',
        started_at: iso(-5),
        expires_at: iso(25),
        created_at: iso(-5),
      },
    }

    await makeMock(state)(page)
    await openHarness(page)
    await expect(page.getByRole('heading', { name: '订阅套餐' })).toBeVisible()
    await expect(page.getByText('AI 用量账单', { exact: true })).toBeVisible()
    await expect(page.getByText('专业版').first()).toBeVisible()
    await expect(page.getByText('zhipu')).toBeVisible()
    await expect(page.getByText('近 7 天趋势')).toBeVisible()

    await page.getByRole('button', { name: '年付' }).click()
    await expect(page.getByText('年付金额以后端套餐配置为准').first()).toBeVisible()

    await page.getByTestId('plan-select-team').click()

    await expect(page.getByText('团队版').first()).toBeVisible()
    await expect(page.locator('span').filter({ hasText: /^年付$/ }).first()).toBeVisible()
    expect(state.currentSubscription?.plan_id).toBe('team')
    expect(state.currentSubscription?.billing_cycle).toBe('annual')
  })
})