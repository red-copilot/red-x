import { expect, test } from '@playwright/test'
import { login, requirePassword, username } from './support'

test.describe('authentication', () => {
  test('redirects an anonymous visitor to login and preserves the target', async ({ page }) => {
    await page.goto('/tasks')
    await expect(page).toHaveURL(/\/login(?:\?|$)/)
    expect(new URL(page.url()).searchParams.get('next')).toBe('/tasks')
    await expect(page.getByRole('heading', { name: '进入攻击面工作区' })).toBeVisible()
    await expect(page.locator('input[autocomplete="username"]')).toHaveValue('admin')
  })

  test('rejects invalid credentials without creating a session', async ({ page }) => {
    await page.goto('/login')
    await page.locator('input[autocomplete="username"]').fill(username)
    await page.locator('input[autocomplete="current-password"]').fill('definitely-not-the-password')
    await page.locator('button[type="submit"]').click()
    await expect(page.getByText('invalid username or password', { exact: true })).toBeVisible()
    await expect(page).toHaveURL(/\/login$/)
    const cookies = await page.context().cookies()
    expect(cookies.find((cookie) => cookie.name === 'arl_access')).toBeUndefined()
  })

  test('signs in, restores the session after reload, and sets expected cookie attributes', async ({ page }) => {
    requirePassword()
    await login(page)
    const cookies = await page.context().cookies()
    const access = cookies.find((cookie) => cookie.name === 'arl_access')
    const csrf = cookies.find((cookie) => cookie.name === 'arl_csrf')
    expect(access?.httpOnly).toBe(true)
    expect(csrf?.httpOnly).toBe(false)
    await page.reload()
    await expect(page.getByRole('heading', { name: '外部攻击面总览' })).toBeVisible()
  })

  test('signs out and protects console routes again', async ({ page }) => {
    await login(page)
    await page.getByRole('button', { name: new RegExp(username) }).click()
    await page.getByText('退出登录', { exact: true }).click()
    await expect(page).toHaveURL(/\/login$/)
    await page.goto('/tasks')
    await expect(page).toHaveURL(/\/login(?:\?|$)/)
    expect(new URL(page.url()).searchParams.get('next')).toBe('/tasks')
  })
})
