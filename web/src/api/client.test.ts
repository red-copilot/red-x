import { afterEach, describe, expect, it, vi } from 'vitest'
import { execute } from './client'

describe('typed API execution', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('refreshes the session once and retries a 401 operation', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 200 }))
    vi.stubGlobal('fetch', fetchMock)
    let calls = 0
    const operation = vi.fn(async () => {
      calls += 1
      if (calls === 1) return { response: new Response(null, { status: 401 }), error: { title: 'expired' } }
      return { response: new Response(null, { status: 200 }), data: { ok: true } }
    })
    await expect(execute(operation)).resolves.toEqual({ ok: true })
    expect(operation).toHaveBeenCalledTimes(2)
    expect(fetchMock).toHaveBeenCalledWith('/api/v1/auth/refresh', expect.objectContaining({ method: 'POST' }))
  })

  it('preserves problem details in ApiError', async () => {
    const operation = vi.fn(async () => ({
      response: new Response(null, { status: 403 }),
      error: { title: 'Forbidden', detail: 'requires operator', status: 403, code: 'forbidden' },
    }))
    await expect(execute(operation)).rejects.toMatchObject({
      status: 403,
      message: 'requires operator',
      problem: expect.objectContaining({ code: 'forbidden' }),
    })
  })
})
