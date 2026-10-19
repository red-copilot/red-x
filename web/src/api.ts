export type Problem = {
  type?: string
  title?: string
  status?: number
  code?: string
  detail?: string
  requestId?: string
}

export class APIError extends Error {
  status: number
  code: string
  requestId?: string
  constructor(status: number, problem: Problem) {
    super(problem.detail || problem.title || `HTTP ${status}`)
    this.status = status
    this.code = problem.code || 'http_error'
    this.requestId = problem.requestId
  }
}

function csrfToken(): string {
  const match = document.cookie.match(/(?:^|; )arl_csrf=([^;]+)/)
  return match ? decodeURIComponent(match[1]) : ''
}

export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers || {})
  if (init.body && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json')
  const method = (init.method || 'GET').toUpperCase()
  if (!['GET', 'HEAD', 'OPTIONS'].includes(method)) {
    const token = csrfToken()
    if (token) headers.set('X-CSRF-Token', token)
  }
  const response = await fetch(`/api/v1${path}`, {
    ...init,
    method,
    headers,
    credentials: 'same-origin',
  })
  const contentType = response.headers.get('content-type') || ''
  const data = contentType.includes('application/json') ? await response.json() : await response.text()
  if (!response.ok) {
    if (response.status === 401 && path !== '/auth/login') {
      window.dispatchEvent(new CustomEvent('arl:unauthorized'))
    }
    throw new APIError(response.status, typeof data === 'object' ? data : { detail: String(data) })
  }
  return data as T
}

export function qs(params: Record<string, unknown>): string {
  const search = new URLSearchParams()
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== null && value !== '') search.set(key, String(value))
  })
  const text = search.toString()
  return text ? `?${text}` : ''
}

export function formatTime(value?: string | null): string {
  if (!value) return '-'
  const d = new Date(value)
  return Number.isNaN(d.getTime()) ? value : d.toLocaleString('zh-CN', { hour12: false })
}
