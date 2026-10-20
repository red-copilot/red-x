import { expect, test } from '@playwright/test'
import { login } from './support'

test('surface overview and primary pages load', async ({ page }) => {
  await login(page)
  await expect(page.getByText('外部攻击面总览')).toBeVisible()
  await expect(page.getByText('风险组织')).toBeVisible()
  await page.getByRole('link', { name: '组织工作区' }).click()
  await expect(page).toHaveURL(/\/organizations$/)
  await expect(page.getByRole('heading', { name: '组织治理' })).toBeVisible()
  await page.getByRole('link', { name: '资产', exact: true }).click()
  await expect(page).toHaveURL(/\/assets$/)
  await expect(page.getByRole('heading', { name: '攻击面资产' })).toBeVisible()
  await page.getByRole('link', { name: '风险', exact: true }).click()
  await expect(page).toHaveURL(/\/findings$/)
  await expect(page.getByRole('heading', { name: '当前风险' })).toBeVisible()
  await page.getByRole('link', { name: '任务', exact: true }).click()
  await expect(page).toHaveURL(/\/tasks$/)
  await expect(page.getByRole('heading', { name: '扫描任务' })).toBeVisible()
})
