import { expect, test } from '@playwright/test'
import { login, trackBrowserErrors } from './support'

const routes = [
  { path: '/dashboard', title: '资产态势' },
  { path: '/tasks', title: '任务' },
  { path: '/assets', title: '资产' },
  { path: '/findings', title: '发现' },
  { path: '/scopes', title: '资产组' },
  { path: '/policies', title: '策略' },
  { path: '/monitors', title: '监控' },
  { path: '/schedules', title: '计划任务' },
  { path: '/github', title: 'GitHub' },
  { path: '/fingerprints', title: '指纹库' },
  { path: '/poc', title: 'PoC 插件' },
  { path: '/system', title: '系统' },
] as const

test.describe('console routes', () => {
  for (const route of routes) {
    test(`${route.path} renders ${route.title} without browser or API errors`, async ({ page }) => {
      await login(page)
      const errors = trackBrowserErrors(page)

      await page.goto(route.path)
      await expect(page.getByRole('heading', { name: route.title, exact: true })).toBeVisible()
      await page.waitForLoadState('networkidle')

      expect(errors).toEqual([])
    })
  }

  test('sidebar navigation reaches tasks, assets, and system pages', async ({ page }) => {
    await login(page)

    await page.getByRole('menuitem', { name: '任务', exact: true }).click()
    await expect(page).toHaveURL(/\/tasks$/)
    await expect(page.getByRole('heading', { name: '任务', exact: true })).toBeVisible()

    await page.getByRole('menuitem', { name: '资产', exact: true }).click()
    await expect(page).toHaveURL(/\/assets$/)
    await expect(page.getByRole('heading', { name: '资产', exact: true })).toBeVisible()

    await page.getByRole('menuitem', { name: '系统', exact: true }).click()
    await expect(page).toHaveURL(/\/system$/)
    await expect(page.getByRole('heading', { name: '系统', exact: true })).toBeVisible()
  })

  test('opens the API documentation from the system page', async ({ page, context }) => {
    await login(page)
    await page.goto('/system')
    const link = page.getByRole('link', { name: 'API 文档', exact: true })
    await expect(link).toHaveAttribute('href', /\/api\/v1\/docs/)
    const [docs] = await Promise.all([context.waitForEvent('page'), link.click()])
    await docs.waitForLoadState()
    await expect(docs).toHaveURL(/\/api\/v1\/docs/)
    await docs.close()
  })
})
