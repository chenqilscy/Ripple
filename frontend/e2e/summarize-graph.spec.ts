import { expect, test } from '@playwright/test'

async function mockSummarizeApi(page: import('@playwright/test').Page) {
  await page.route('**/api/v1/**', async route => {
    const url = new URL(route.request().url())
    const method = route.request().method()

    if (url.pathname === '/api/v1/lakes/lake-e2e/nodes/summarize' && method === 'POST') {
      const payload = JSON.parse(route.request().postData() ?? '{}') as { node_ids?: string[]; title_hint?: string }
      expect(payload.node_ids).toEqual(['node-a', 'node-b'])
      expect(payload.title_hint).toBe('聚焦产品价值')
      await route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify({
          summary_node: { id: 'summary-node-123456', content: '综合摘要：两个节点共同指向更清晰的产品价值闭环。' },
          sources: [
            { id: 'node-a', content_snippet: '业务目标：降低首次使用门槛。', content_length: 16 },
            { id: 'node-b', content_snippet: '技术方案：用 AI 整理多节点关系。', content_length: 19 },
          ],
          edges: [
            { source_id: 'summary-node-123456', target_id: 'node-a', kind: 'summarizes' },
            { source_id: 'summary-node-123456', target_id: 'node-b', kind: 'summarizes' },
          ],
          edge_failures: [],
          source_count: 2,
          edge_kind: 'summarizes',
          complete: true,
        }),
      })
      return
    }

    throw new Error(`Unhandled API request in summarize-graph.spec.ts: ${method} ${url.pathname}`)
  })
}

async function openHarness(page: import('@playwright/test').Page) {
  await page.addInitScript(() => window.localStorage.setItem('ripple.token', 'e2e-token'))
  await page.goto('/summarize-graph-harness.html')
  await expect(page.getByRole('heading', { name: 'Summarize Graph Harness' })).toBeVisible({ timeout: 10_000 })
}

test.describe('Summarize graph modal', () => {
  test('展示整理前后预览与 summarizes 关联结果', async ({ page }) => {
    await mockSummarizeApi(page)
    await openHarness(page)

    await expect(page.getByRole('heading', { name: '多节点 AI 整理' })).toBeVisible()
    await page.getByPlaceholder('让 AI 聚焦于某个角度，如"分析技术可行性"（可留空）').fill('聚焦产品价值')
    await page.getByRole('button', { name: '生成整理结果 (2 节点)' }).click()

    await expect(page.getByText('整理前：源节点预览')).toBeVisible()
    await expect(page.getByText('整理后：新摘要节点')).toBeVisible()
    await expect(page.getByText('综合摘要：两个节点共同指向更清晰的产品价值闭环。')).toBeVisible()
    await expect(page.getByText('业务目标：降低首次使用门槛。')).toBeVisible()
    await expect(page.getByText('摘要关联预览')).toBeVisible()
    await expect(page.getByText(/summarizes/).first()).toBeVisible()

    await page.getByRole('button', { name: '关闭并刷新图谱' }).click()
    await expect(page.getByTestId('success-count')).toHaveText('success:1')
  })
})
