import { expect, test } from '@playwright/test'

async function mockAiTriggerApi(page: import('@playwright/test').Page) {
  let statusCalls = 0
  await page.route('**/api/v1/**', async route => {
    const url = new URL(route.request().url())
    const method = route.request().method()

    if (url.pathname === '/api/v1/prompt_templates' && method === 'GET') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          items: [
            {
              id: 'tpl-e2e',
              name: '学习笔记模板',
              description: '整理当前节点',
              template: '请整理 {{node_content}}',
              scope: 'private',
              created_by: 'u-e2e',
              created_at: new Date().toISOString(),
              updated_at: new Date().toISOString(),
            },
          ],
          total: 1,
        }),
      })
      return
    }

    if (url.pathname === '/api/v1/lakes/lake-e2e/nodes/node-e2e/ai_trigger' && method === 'POST') {
      const payload = JSON.parse(route.request().postData() ?? '{}') as { prompt_template_id?: string }
      expect(payload.prompt_template_id).toBe('tpl-e2e')
      await route.fulfill({
        status: 202,
        contentType: 'application/json',
        body: JSON.stringify({
          job_id: 'job-e2e',
          ai_job_id: 'job-e2e',
          node_id: 'node-e2e',
          status: 'pending',
          progress_pct: 0,
          estimated_seconds: 15,
        }),
      })
      return
    }

    if (url.pathname === '/api/v1/lakes/lake-e2e/nodes/node-e2e/ai_status' && method === 'GET') {
      statusCalls += 1
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          job_id: 'job-e2e',
          ai_job_id: 'job-e2e',
          node_id: 'node-e2e',
          status: statusCalls >= 1 ? 'done' : 'processing',
          progress_pct: 100,
          started_at: new Date().toISOString(),
          finished_at: new Date().toISOString(),
        }),
      })
      return
    }

    throw new Error(`Unhandled API request in ai-trigger.spec.ts: ${method} ${url.pathname}`)
  })
}

test.describe('AI trigger node detail entry', () => {
  test('可从节点详情选择 Prompt 模板并跟踪完成状态', async ({ page }) => {
    await mockAiTriggerApi(page)
    await page.addInitScript(() => window.localStorage.setItem('ripple.token', 'e2e-token'))
    await page.goto('/ai-trigger-harness.html')

    await expect(page.getByRole('heading', { name: 'AI Trigger Harness' })).toBeVisible()
    await expect(page.getByText('AI Workflow')).toBeVisible()

    await page.getByLabel('选择 Prompt 模板').selectOption('tpl-e2e')
    await page.getByRole('button', { name: /AI 触发/ }).click()

    await expect(page.getByText('完成', { exact: true })).toBeVisible({ timeout: 5_000 })
    await expect(page.getByTestId('success-count')).toHaveText('success:1')
  })
})
