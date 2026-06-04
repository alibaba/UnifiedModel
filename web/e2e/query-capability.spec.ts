import { test, expect } from '@playwright/test'

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
    await page.getByRole('button', { name: '.umodel' }).click()
    await page.getByRole('button', { name: 'Execute' }).click()

    await expect(page.locator('.om-table tbody tr').first()).toBeVisible({ timeout: 10_000 })
    const rowCount = await page.locator('.om-table tbody tr').count()
    expect(rowCount).toBeGreaterThanOrEqual(10)
  })

  test('entity query finds checkout service', async ({ page }) => {
    await page.getByRole('button', { name: 'Query' }).click()
    await page.getByRole('button', { name: '.entity' }).click()
    await page.getByRole('button', { name: 'Execute' }).click()

    await expect(page.locator('.om-table')).toBeVisible({ timeout: 10_000 })
    await expect(page.locator('.om-table').locator('text=checkout-service').first()).toBeVisible()
  })

  test('topo query returns direct relations', async ({ page }) => {
    await page.getByRole('button', { name: 'Query' }).click()
    await page.getByRole('button', { name: 'direct' }).click()
    await page.getByRole('button', { name: 'Execute' }).click()

    await expect(page.locator('.om-table tbody tr').first()).toBeVisible({ timeout: 10_000 })
  })

  test('explain shows provider info', async ({ page }) => {
    await page.getByRole('button', { name: 'Query' }).click()
    await page.getByRole('button', { name: '.umodel' }).click()
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

    await page.getByRole('button', { name: 'Call' }).click()
    await expect(page.locator('.api-debug-response', { hasText: 'query_spl_execute' })).toBeVisible({ timeout: 10_000 })
  })
})
