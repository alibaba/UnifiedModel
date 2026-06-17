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

  test('submit delete success closes preview without Monaco model errors', async ({ page }) => {
    const browserErrors: string[] = []
    page.on('pageerror', (error) => browserErrors.push(error.message))
    page.on('console', (message) => {
      if (message.type() === 'error') browserErrors.push(message.text())
    })
    await page.route('**/api/v1/umodel/demo/elements', async (route) => {
      if (route.request().method() !== 'DELETE') {
        await route.continue()
        return
      }

      const body = route.request().postDataJSON() as { ids?: string[] }
      const id = body.ids?.[0] || 'unknown'
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          accepted: 1,
          failed: 0,
          items: [{
            id,
            ok: true,
          }],
        }),
      })
    })

    await page.getByRole('button', { name: 'Table' }).click()
    await expect(page.locator('.ume-data-table tbody tr').first()).toBeVisible({ timeout: 10_000 })

    const linkRow = page.locator('.ume-data-table tbody tr', { hasText: 'EntitySetLink' }).first()
    await expect(linkRow).toBeVisible()
    await linkRow.locator('.ume-table-delete-button').click()

    await page.getByRole('button', { name: /^Submit/ }).click()
    await expect(page.locator('.ume-diff-drawer')).toBeVisible()
    await page.getByRole('button', { name: 'Confirm & Submit' }).click()
    await expect(page.locator('.ume-diff-drawer')).toBeHidden({ timeout: 10_000 })
    await page.waitForTimeout(250)

    expect(browserErrors.filter((message) => message.includes('TextModel got disposed before DiffEditorWidget model got reset'))).toEqual([])
  })

  test('submit preview surfaces delete write failures', async ({ page }) => {
    await page.route('**/api/v1/umodel/demo/elements', async (route) => {
      if (route.request().method() !== 'DELETE') {
        await route.continue()
        return
      }

      const body = route.request().postDataJSON() as { ids?: string[] }
      const id = body.ids?.[0] || 'unknown'
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          accepted: 0,
          failed: 1,
          items: [{
            id,
            ok: false,
            code: 'NOT_IMPLEMENTED',
            message: 'delete dependency checks are not implemented in the current service',
          }],
        }),
      })
    })

    await page.getByRole('button', { name: 'Table' }).click()
    await expect(page.locator('.ume-data-table tbody tr').first()).toBeVisible({ timeout: 10_000 })

    const linkRow = page.locator('.ume-data-table tbody tr', { hasText: 'EntitySetLink' }).first()
    await expect(linkRow).toBeVisible()
    await linkRow.locator('.ume-table-delete-button').click()
    await expect(page.getByRole('button', { name: /^Submit/ })).toBeEnabled()

    await page.getByRole('button', { name: /^Submit/ }).click()
    await expect(page.locator('text=Submit Preview')).toBeVisible()
    const drawerBox = await page.locator('.ume-diff-drawer').boundingBox()
    const confirmBox = await page.getByRole('button', { name: 'Confirm & Submit' }).boundingBox()
    expect(drawerBox).not.toBeNull()
    expect(confirmBox).not.toBeNull()
    expect(confirmBox!.y).toBeGreaterThan(drawerBox!.y + drawerBox!.height - 120)

    await page.getByRole('button', { name: 'Confirm & Submit' }).click()
    await expect(page.locator('.ume-submit-failure')).toContainText('Delete accepted 0 item(s), failed 1 item(s).')
    await expect(page.locator('.ume-submit-failure')).toContainText('NOT_IMPLEMENTED')
    await expect(page.locator('.ume-submit-failure')).toContainText('delete dependency checks are not implemented')
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
