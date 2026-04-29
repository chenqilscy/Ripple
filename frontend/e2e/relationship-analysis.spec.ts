import { expect, test, type Page } from '@playwright/test'

const lake = {
  id: 'lake-rel',
  name: '关系分析湖',
  description: '',
  is_public: false,
  owner_id: 'user-e2e',
  role: 'OWNER',
}

const nodes = [
  {
    id: 'node-a',
    lake_id: lake.id,
    owner_id: 'user-e2e',
    content: '小龙虾产业链中的经济与社会影响',
    type: 'TEXT',
    state: 'MIST',
    position: { x: 0, y: 0, z: 0 },
    created_at: '2026-04-29T00:00:00Z',
    updated_at: '2026-04-29T00:00:00Z',
  },
  {
    id: 'node-b',
    lake_id: lake.id,
    owner_id: 'user-e2e',
    content: '中国小龙虾产业的崛起与发展',
    type: 'TEXT',
    state: 'MIST',
    position: { x: 30, y: 0, z: 0 },
    created_at: '2026-04-29T00:00:00Z',
    updated_at: '2026-04-29T00:00:00Z',
  },
] as const

async function mockRelationshipApi(page: Page) {
  const edges: unknown[] = []
  let createEdgeCount = 0

  await page.route('**/api/v1/**', async route => {
    const url = new URL(route.request().url())
    const method = route.request().method()
    const ok = (body: unknown, status = 200) => route.fulfill({
      status,
      contentType: 'application/json',
      body: JSON.stringify(body),
    })

    if (url.pathname === '/api/v1/auth/me' && method === 'GET') {
      await ok({ id: 'user-e2e', email: 'e2e@ripple.test', display_name: 'E2E' })
      return
    }
    if (url.pathname === '/api/v1/spaces' && method === 'GET') {
      await ok({ spaces: [] })
      return
    }
    if (url.pathname === '/api/v1/lakes' && method === 'GET') {
      await ok({ lakes: [lake] })
      return
    }
    if (url.pathname === '/api/v1/recommendations' && method === 'GET') {
      await ok({ recommendations: [] })
      return
    }
    if (url.pathname === '/api/v1/notifications/unread_count' && method === 'GET') {
      await ok({ count: 0 })
      return
    }
    if (url.pathname === '/api/v1/ws_token' && method === 'POST') {
      await ok({ token: 'ws-e2e-token', expires_in: 300 })
      return
    }
    if (url.pathname === `/api/v1/lakes/${lake.id}/nodes` && method === 'GET') {
      await ok({ nodes })
      return
    }
    if (url.pathname === `/api/v1/lakes/${lake.id}/edges` && method === 'GET') {
      await ok({ edges })
      return
    }
    if (url.pathname === `/api/v1/lakes/${lake.id}/presence` && method === 'GET') {
      await ok({ users: [] })
      return
    }
    if (url.pathname === `/api/v1/lakes/${lake.id}/tags` && method === 'GET') {
      await ok({ tags: [] })
      return
    }
    if (/^\/api\/v1\/nodes\/node-[ab]\/tags$/.test(url.pathname) && method === 'GET') {
      await ok({ tags: [] })
      return
    }
    if (url.pathname === '/api/v1/nodes/node-a/related' && method === 'GET') {
      await ok({ related: [{ node_id: 'node-b', lake_id: lake.id, snippet: nodes[1].content, score: 8.75 }] })
      return
    }
    if (url.pathname === '/api/v1/nodes/node-b/related' && method === 'GET') {
      await ok({ related: [{ node_id: 'node-a', lake_id: lake.id, snippet: nodes[0].content, score: 7.25 }] })
      return
    }
    if (url.pathname === '/api/v1/edges' && method === 'POST') {
      const payload = JSON.parse(route.request().postData() ?? '{}') as { src_node_id?: string; dst_node_id?: string; kind?: string; label?: string }
      expect(payload).toMatchObject({ src_node_id: 'node-a', dst_node_id: 'node-b', kind: 'relates' })
      createEdgeCount += 1
      const edge = {
        id: `edge-${createEdgeCount}`,
        lake_id: lake.id,
        src_node_id: payload.src_node_id,
        dst_node_id: payload.dst_node_id,
        kind: payload.kind,
        label: payload.label,
        owner_id: 'user-e2e',
        created_at: '2026-04-29T00:00:00Z',
      }
      edges.push(edge)
      await ok(edge, 201)
      return
    }

    throw new Error(`Unhandled API request in relationship-analysis.spec.ts: ${method} ${url.pathname}`)
  })

  return { getCreateEdgeCount: () => createEdgeCount }
}

test.describe('Relationship analysis graph flow', () => {
  test('手动分析关系会创建 relates 边并切到图谱', async ({ page }) => {
    const apiProbe = await mockRelationshipApi(page)
    await page.addInitScript(() => window.localStorage.setItem('ripple.token', 'e2e-token'))

    await page.goto('/')
    await expect(page.getByRole('heading', { name: lake.name })).toBeVisible({ timeout: 10_000 })

    await page.getByRole('button', { name: '🔎 分析关系' }).click()

    await expect(page.getByText('正在分析全湖节点关系…')).toBeVisible()
    await expect(page.getByText('关系分析完成：新增 1 条关联。').first()).toBeVisible({ timeout: 10_000 })
    await expect(page.getByRole('button', { name: '✓ 图谱' })).toBeVisible()
    await expect(page.getByText(/relates: 自动关联/)).toBeVisible()
    expect(apiProbe.getCreateEdgeCount()).toBe(1)

    const dismiss = page.getByRole('button', { name: '知道了' })
    if (await dismiss.isVisible().catch(() => false)) await dismiss.click()
  })
})
