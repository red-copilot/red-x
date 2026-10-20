import { expect, test } from '@playwright/test'
import { login, trackBrowserErrors } from './support'

const routes = [
  { path: '/', title: '外部攻击面总览' },
  { path: '/organizations', title: '组织治理' },
  { path: '/assets', title: '攻击面资产' },
  { path: '/findings', title: '当前风险' },
  { path: '/tasks', title: '扫描任务' },
  { path: '/automation/monitors', title: '周期监控' },
  { path: '/automation/schedules', title: '计划任务' },
  { path: '/automation/policies', title: '扫描策略' },
  { path: '/automation/groups', title: '资产组' },
  { path: '/intelligence/github', title: 'GitHub 情报' },
  { path: '/intelligence/fingerprints', title: '指纹库' },
  { path: '/intelligence/poc', title: 'PoC 插件' },
  { path: '/admin/users', title: '用户管理' },
  { path: '/admin/api-keys', title: 'API Key' },
  { path: '/admin/audit', title: '审计日志' },
  { path: '/admin/system', title: '系统' },
] as const

test.describe('console routes', () => {
  for (const route of routes) {
    test(`${route.path} renders ${route.title} without browser or API errors`, async ({ page }) => {
      await login(page)
      if (route.path !== '/') await page.goto(route.path)
      const errors = trackBrowserErrors(page)
      await expect(page.getByRole('heading', { name: route.title, exact: true })).toBeVisible()
      await page.waitForLoadState('networkidle')
      expect(errors).toEqual([])
    })
  }

  test('opens global search with the keyboard', async ({ page }) => {
    await login(page)
    await page.keyboard.press('Control+K')
    await expect(page.getByPlaceholder('搜索组织、域名、资产、风险或任务…')).toBeVisible()
  })

  test('returns a 404 page for removed routes', async ({ page }) => {
    await login(page)
    await page.goto('/dashboard')
    await expect(page.getByRole('heading', { name: '页面不存在' })).toBeVisible()
  })
})
