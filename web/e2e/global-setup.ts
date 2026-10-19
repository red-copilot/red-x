import { request } from '@playwright/test'

export default async function globalSetup() {
  const baseURL = process.env.E2E_BASE_URL || 'http://127.0.0.1:5003'
  const context = await request.newContext({ baseURL, ignoreHTTPSErrors: true })
  try {
    const response = await context.get('/readyz', { timeout: 10_000 })
    if (!response.ok()) {
      throw new Error(`ARL is not ready at ${baseURL}: /readyz returned HTTP ${response.status()}`)
    }
  } catch (error) {
    throw new Error(
      `Cannot reach ARL at ${baseURL}. Start the stack (for example: docker compose -f deploy/compose.yaml up -d) or set E2E_BASE_URL. Cause: ${String(error)}`,
    )
  } finally {
    await context.dispose()
  }
}
