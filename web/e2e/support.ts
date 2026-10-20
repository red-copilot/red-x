import { expect, type Page } from '@playwright/test'

export const username = process.env.E2E_USERNAME || 'admin'
export const password = process.env.E2E_PASSWORD || ''

export function requirePassword() {
  if (!password) {
    throw new Error('E2E_PASSWORD is required. Set it to the password for E2E_USERNAME.')
  }
}

async function submitLogin(page: Page) {
  await page.locator('input[autocomplete="username"]').fill(username)
  await page.locator('input[autocomplete="current-password"]').fill(password)
  await page.locator('button[type="submit"]').click()
}

export async function login(page: Page) {
  requirePassword()
  await page.goto('/login')
  await submitLogin(page)
  await page.waitForURL((url) => url.pathname === '/' || url.pathname === '/change-password')

  if (new URL(page.url()).pathname === '/change-password') {
    await page.locator('#oldPassword').fill(password)
    await page.locator('#newPassword').fill(password)
    await page.locator('#confirmPassword').fill(password)
    await page.getByRole('button', { name: '修改密码' }).click()
    await expect(page).toHaveURL(/\/login$/)
    await submitLogin(page)
    await page.waitForURL((url) => url.pathname === '/')
  }

  await expect(page.getByRole('heading', { name: '外部攻击面总览' })).toBeVisible()
}

export function trackBrowserErrors(page: Page) {
  const errors: string[] = []
  page.on('pageerror', (error) => errors.push(`pageerror: ${error.message}`))
  page.on('console', (message) => {
    if (message.type() === 'error') errors.push(`console: ${message.text()}`)
  })
  page.on('response', (response) => {
    const url = new URL(response.url())
    if (url.pathname.startsWith('/api/') && response.status() >= 400 && !url.pathname.endsWith('/auth/me')) {
      errors.push(`api: HTTP ${response.status()} ${url.pathname}`)
    }
  })
  return errors
}
