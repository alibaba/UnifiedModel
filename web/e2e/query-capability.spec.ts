import { test, expect, type Page } from '@playwright/test'

async function pickQueryExample(page: Page, name: string) {
  await page.locator('.query-example-trigger').click()
  await page.getByRole('menuitem', { name }).click()
}

test.describe('Query capability via UI', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/')
    const demoCard = page.locator('.workspace-id', { hasText: 'demo' }).first()
    await expect(demoCard).toBeVisible({ timeout: 10_000 })
    await demoCard.click()
    await expect(page.locator('text=UModel Explorer')).toBeVisible({ timeout: 5_000 })
  })

  test('workspace landing shows demo workspace', async ({ page }) => {
    await page.goto('/')
    await expect(page.locator('.workspace-id', { hasText: 'demo' }).first()).toBeVisible({ timeout: 10_000 })
  })

  test('umodel query returns rows', async ({ page }) => {
    await page.getByRole('button', { name: 'Query' }).click()
    await expect(page.getByRole('button', { name: 'Execute' })).toBeVisible()
    await pickQueryExample(page, '.umodel')
    await page.getByRole('button', { name: 'Execute' }).click()

    await expect(page.locator('.om-table tbody tr').first()).toBeVisible({ timeout: 10_000 })
    const rowCount = await page.locator('.om-table tbody tr').count()
    expect(rowCount).toBeGreaterThanOrEqual(10)
  })

  test('entity query finds checkout service', async ({ page }) => {
    await page.getByRole('button', { name: 'Query' }).click()
    await pickQueryExample(page, '.entity')
    await page.getByRole('button', { name: 'Execute' }).click()

    await expect(page.locator('.om-table')).toBeVisible({ timeout: 10_000 })
    await expect(page.locator('.om-table').locator('text=checkout-service').first()).toBeVisible()
  })

  test('topo query returns direct relations', async ({ page }) => {
    await page.getByRole('button', { name: 'Query' }).click()
    await pickQueryExample(page, 'direct')
    await page.getByRole('button', { name: 'Execute' }).click()

    await expect(page.locator('.om-table tbody tr').first()).toBeVisible({ timeout: 10_000 })
  })

  test('explain shows provider info', async ({ page }) => {
    await page.getByRole('button', { name: 'Query' }).click()
    await pickQueryExample(page, '.umodel')
    await page.getByRole('button', { name: 'Explain' }).click()

    await expect(page.locator('text=memory')).toBeVisible({ timeout: 10_000 })
  })

  test('explorer view renders graph', async ({ page }) => {
    await expect(page.locator('text=UModel Explorer')).toBeVisible()
    await page.waitForTimeout(2_000)
    const hasContent = await page.locator('.react-flow, [data-testid="explorer"], canvas, svg').first().isVisible().catch(() => false)
    expect(hasContent || (await page.locator('text=entity_set').count()) > 0).toBeTruthy()
  })

  test('api debugger shows agent discovery tools', async ({ page }) => {
    await page.getByRole('button', { name: 'API Debugger' }).click()
    await expect(page).toHaveURL(/\/workspaces\/demo\/api-debug$/)

    await page.locator('.api-debug-list button', { hasText: 'Discover agent surface' }).click()
    await expect(page.locator('.api-debug-request-line code', { hasText: '/api/v1/agent/demo/discover' })).toBeVisible()

    const responsePromise = page.waitForResponse((response) => (
      response.request().method() === 'GET' &&
      response.url().includes('/api/v1/agent/demo/discover')
    ))
    await page.getByRole('button', { name: 'Call' }).click()
    const response = await responsePromise
    expect(response.ok()).toBeTruthy()
    const payload = await response.json() as { tools?: Array<{ name?: string }> }
    expect(payload.tools?.map((tool) => tool.name)).toContain('query_spl_execute')
    await expect(page.locator('.api-debug-response', { hasText: '200 OK' })).toBeVisible({ timeout: 10_000 })
  })
})
