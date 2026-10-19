import { expect, test } from '@playwright/test'
import { login, trackBrowserErrors } from './support'

test('validates a new task without creating one', async ({ page }) => {
  await login(page)
  const errors = trackBrowserErrors(page)
  let createRequests = 0
  page.on('request', (request) => {
    if (request.method() === 'POST' && new URL(request.url()).pathname === '/api/v1/tasks') {
      createRequests += 1
    }
  })

  await page.goto('/tasks')
  await page.getByRole('button', { name: '新建任务' }).click()
  const dialog = page.getByRole('dialog')
  await expect(dialog).toBeVisible()
  await expect(dialog.getByText('新建任务', { exact: true })).toBeVisible()

  await dialog.locator('.ant-modal-footer .ant-btn-primary').click()
  await expect(page.getByText('请输入目标', { exact: true })).toBeVisible()
  expect(createRequests).toBe(0)

  await page.keyboard.press('Escape')
  await expect(dialog).toBeHidden()
  expect(errors).toEqual([])
})
