import { expect, test } from '@playwright/test'

test('UModel Explorer requests up to 2000 model elements without exceeding the request limit contract', async ({ page }) => {
  const queryRequests: Array<{ query?: string; limit?: number }> = []

  await page.route('**/healthz', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        status: 'ok',
        graphstore: { provider: 'memory', status: 'ok' },
      }),
    })
  })
  await page.route('**/api/v1/workspaces/demo', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        id: 'demo',
        name: 'Demo',
        paths: { root: '/tmp/demo' },
        status: 'active',
        resource_version: 1,
        created_at: '2026-08-10T00:00:00Z',
        updated_at: '2026-08-10T00:00:00Z',
      }),
    })
  })
  await page.route('**/api/v1/query/demo/execute', async (route) => {
    queryRequests.push(route.request().postDataJSON() as { query?: string; limit?: number })
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        columns: [],
        rows: [],
        page: { limit: queryRequests.at(-1)?.limit },
      }),
    })
  })

  await page.goto('/workspaces/demo/umodel')

  await expect.poll(() => queryRequests.length).toBeGreaterThan(0)
  expect(queryRequests[0]).toEqual({
    query: '.umodel | sort name | limit 2000',
  })
})
