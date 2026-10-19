import { expect, type Page } from '@playwright/test'

export const username = process.env.E2E_USERNAME || 'admin'
export const password = process.env.E2E_PASSWORD || ''

export function requirePassword() {
  if (!password) {
    throw new Error('E2E_PASSWORD is required. Set it to the password for E2E_USERNAME.')
  }
}

export async function login(page: Page) {
  requirePassword()
  await page.goto('/login')
  await page.locator('input[autocomplete="username"]').fill(username)
  await page.locator('input[autocomplete="current-password"]').fill(password)
  await page.locator('button[type="submit"]').click()
  await expect(page).toHaveURL(/\/dashboard$/)
  await expect(page.getByRole('heading', { name: '资产态势' })).toBeVisible()
}

export function trackBrowserErrors(page: Page) {
  const errors: string[] = []
  page.on('pageerror', (error) => errors.push(`pageerror: ${error.message}`))
  page.on('console', (message) => {
    if (message.type() === 'error') errors.push(`console: ${message.text()}`)
  })
  page.on('response', (response) => {
    const url = new URL(response.url())
    if (url.pathname.startsWith('/api/') && response.status() >= 400) {
      errors.push(`api: HTTP ${response.status()} ${url.pathname}`)
    }
  })
  return errors
}
