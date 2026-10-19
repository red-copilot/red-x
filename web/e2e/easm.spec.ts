import { expect, test } from '@playwright/test'

const username = process.env.E2E_USERNAME || 'admin'
const password = process.env.E2E_PASSWORD || ''

test('EASM wallboard loads, selects an organization, and toggles the graph', async ({ page }) => {
  test.skip(!password, 'E2E_PASSWORD is required')
  await page.goto('/login')
  await page.locator('input[autocomplete="username"]').fill(username)
  await page.locator('input[autocomplete="current-password"]').fill(password)
  await page.locator('button[type="submit"]').click()

  await expect(page).toHaveURL(/\/easm$/)
  await expect(page.getByText('外部攻击面态势')).toBeVisible()
  await expect(page.locator('.easm-page')).toBeVisible()
  await expect(page.getByText(/ICP ·/)).toBeVisible()

  const orgButton = page.locator('aside.rail button.org').first()
  if (await orgButton.count()) {
    await orgButton.click()
    await expect(page.locator('aside.inspector')).toBeVisible()
    await page.getByRole('button', { name: '图谱' }).click()
  }
})
