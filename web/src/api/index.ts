import type { components } from './schema'
import { client, execute } from './client'

export type User = components['schemas']['User']
export type Organization = components['schemas']['Organization']
export type OrganizationDomain = components['schemas']['OrganizationDomain']
export type OrganizationSummary = components['schemas']['OrganizationSummary']
export type EasmAsset = components['schemas']['EasmAsset']
export type AssetPage = components['schemas']['AssetPage']
export type Finding = components['schemas']['Finding']
export type FindingItem = components['schemas']['FindingItem']
export type FindingPage = components['schemas']['FindingPage']
export type Task = components['schemas']['Task']
export type TaskList = components['schemas']['TaskList']
export type TaskStep = components['schemas']['TaskStep']
export type TaskDetail = components['schemas']['TaskDetail']
export type Observation = components['schemas']['Observation']
export type Overview = components['schemas']['Overview']
export type SearchResult = components['schemas']['SearchResult']
export type MapSnapshot = components['schemas']['MapSnapshot']
export type GraphSnapshot = components['schemas']['GraphSnapshot']
export type Health = components['schemas']['Health']
export type ICPJob = components['schemas']['ICPJob']
export type Policy = components['schemas']['Policy']
export type Scope = components['schemas']['Scope']
export type Monitor = components['schemas']['Monitor']
export type Schedule = components['schemas']['Schedule']
export type Fingerprint = components['schemas']['Fingerprint']
export type PocPlugin = components['schemas']['PocPlugin']
export type GitHubTask = components['schemas']['GitHubTask']
export type GitHubMonitor = components['schemas']['GitHubMonitor']
export type GitHubFinding = components['schemas']['GitHubFinding']
export type APIKey = components['schemas']['APIKey']
export type AuditEvent = components['schemas']['AuditEvent']
export type SystemInfo = components['schemas']['SystemInfo']

export type PageQuery = {
  page?: number
  pageSize?: number
  q?: string
}

export const api = {
  me: () => execute(() => client.GET('/auth/me')),
  login: (username: string, password: string) =>
    execute(() => client.POST('/auth/login', { body: { username, password } }), false),
  logout: () => execute(() => client.POST('/auth/logout')),
  changePassword: (oldPassword: string, newPassword: string) =>
    execute(() => client.POST('/auth/password', { body: { oldPassword, newPassword } }), false),
  overview: (range = '30d') => execute(() => client.GET('/overview', { params: { query: { range: range as never } } })),
  search: (q: string) => execute(() => client.GET('/search', { params: { query: { q, limit: 5 } } })),
  organizations: (query?: string) =>
    execute(() => client.GET('/organizations', { params: { query: { query, limit: 200 } } })),
  organization: (id: string) => execute(() => client.GET('/organizations/{id}', { params: { path: { id } } })),
  updateOrganization: (id: string, body: Record<string, unknown>) =>
    execute(() => client.PATCH('/organizations/{id}', { params: { path: { id } }, body: body as never })),
  deleteOrganization: (id: string) => execute(() => client.DELETE('/organizations/{id}', { params: { path: { id } } })),
  organizationSummary: (id: string) =>
    execute(() => client.GET('/organizations/{id}/summary', { params: { path: { id } } })),
  domains: (id: string) => execute(() => client.GET('/organizations/{id}/domains', { params: { path: { id } } })),
  updateDomain: (id: string, domainId: string, body: Record<string, unknown>) =>
    execute(() =>
      client.PATCH('/organizations/{id}/domains/{domainId}', {
        params: { path: { id, domainId } },
        body: body as never,
      }),
    ),
  deleteDomain: (id: string, domainId: string) =>
    execute(() => client.DELETE('/organizations/{id}/domains/{domainId}', { params: { path: { id, domainId } } })),
  assets: (query: Record<string, string | number | undefined>) =>
    execute(() => client.GET('/assets', { params: { query: query as never } })),
  asset: (id: string, view = 'surface', organizationId?: string) =>
    execute(() =>
      client.GET('/assets/{id}', { params: { path: { id }, query: { view: view as never, organizationId } } }),
    ),
  createAsset: (body: Record<string, unknown>) => execute(() => client.POST('/assets', { body: body as never })),
  updateAsset: (id: string, body: Record<string, unknown>) =>
    execute(() => client.PATCH('/assets/{id}', { params: { path: { id } }, body: body as never })),
  deleteAsset: (id: string) => execute(() => client.DELETE('/assets/{id}', { params: { path: { id } } })),
  findings: (query: Record<string, string | number | undefined>) =>
    execute(() => client.GET('/findings', { params: { query: query as never } })),
  findingHistory: (id: string, organizationId: string) =>
    execute(() => client.GET('/findings/{id}/history', { params: { path: { id }, query: { organizationId } } })),
  tasks: (query: Record<string, string | number | undefined>) =>
    execute(() => client.GET('/tasks', { params: { query: query as never } })),
  task: (id: string) => execute(() => client.GET('/tasks/{id}', { params: { path: { id } } })),
  taskObservations: (id: string, kind?: string) =>
    execute(() =>
      client.GET('/tasks/{id}/observations', { params: { path: { id }, query: { kind, pageSize: 100 } as never } }),
    ),
  taskFindings: (id: string, kind?: string) =>
    execute(() =>
      client.GET('/tasks/{id}/findings', { params: { path: { id }, query: { kind, pageSize: 100 } as never } }),
    ),
  createTask: (body: Record<string, unknown>) => execute(() => client.POST('/tasks', { body: body as never })),
  cancelTask: (id: string) =>
    execute(() => client.POST('/tasks/{id}/cancel', { params: { path: { id } } as never, body: {} as never })),
  retryTask: (id: string) =>
    execute(() => client.POST('/tasks/{id}/retry', { params: { path: { id } } as never, body: {} as never })),
  syncTask: (id: string) =>
    execute(() => client.POST('/tasks/{id}/sync', { params: { path: { id } } as never, body: {} as never })),
  createExport: (body: Record<string, unknown>) => execute(() => client.POST('/exports', { body: body as never })),
  map: (organizationId?: string, risk?: string) =>
    execute(() => client.GET('/map', { params: { query: { orgId: organizationId, risk } } })),
  graph: (organizationId: string) =>
    execute(() =>
      client.GET('/organizations/{id}/graph', {
        params: { path: { id: organizationId }, query: { depth: 2, limit: 300 } },
      }),
    ),
  organizationAssets: (id: string, query: Record<string, string | number | undefined>) =>
    execute(() => client.GET('/organizations/{id}/assets', { params: { path: { id }, query: query as never } })),
  organizationFindings: (id: string, query: Record<string, string | number | undefined>) =>
    execute(() => client.GET('/organizations/{id}/findings', { params: { path: { id }, query: query as never } })),
  organizationTasks: (id: string, query: Record<string, string | number | undefined>) =>
    execute(() => client.GET('/organizations/{id}/tasks', { params: { path: { id }, query: query as never } })),
  createOrganization: (body: Record<string, unknown>) =>
    execute(() => client.POST('/organizations', { body: body as never })),
  importOrganizations: async (file: File) => {
    const form = new FormData()
    form.set('file', file)
    const response = await fetch('/api/v1/organizations/import', {
      method: 'POST',
      credentials: 'include',
      headers: { 'X-CSRF-Token': csrfFromCookie() },
      body: form,
    })
    if (!response.ok) throw new Error((await response.json().catch(() => ({}))).detail || '导入失败')
    return response.json()
  },
  confirmDomains: (id: string, domains: string[]) =>
    execute(() => client.POST('/organizations/{id}/domains/confirm', { params: { path: { id } }, body: { domains } })),
  importDomains: (id: string, domains: string[]) =>
    execute(() => client.POST('/organizations/{id}/domains/import', { params: { path: { id } }, body: { domains } })),
  rejectDomain: (id: string, domainId: string, reason: string) =>
    execute(() =>
      client.POST('/organizations/{id}/domains/{domainId}/reject', {
        params: { path: { id, domainId } },
        body: { reason },
      }),
    ),
  reassignDomain: (id: string, domainId: string, targetOrganizationId: string, reason: string) =>
    execute(() =>
      client.POST('/organizations/{id}/domains/{domainId}/reassign', {
        params: { path: { id, domainId } },
        body: { targetOrganizationId, reason },
      }),
    ),
  evidence: (id: string, domainId: string) =>
    execute(() =>
      client.GET('/organizations/{id}/domains/{domainId}/evidence', { params: { path: { id, domainId } } }),
    ),
  refreshICP: (id: string) =>
    execute(() =>
      client.POST('/organizations/{id}/icp-refresh', { params: { path: { id } } as never, body: {} as never }),
    ),
  lookupDomain: (domain: string) => execute(() => client.POST('/domain-lookups', { body: { domain } })),
  createScans: (id: string, rootDomains: string[], policyId?: string, scopeId?: string) =>
    execute(() =>
      client.POST('/organizations/{id}/scans', {
        params: { path: { id } },
        body: { rootDomains, policyId, scopeId, options: {} },
      }),
    ),
  exportOrganization: (id: string, format: 'xlsx' | 'csv') =>
    execute(() => client.POST('/organizations/{id}/exports', { params: { path: { id } }, body: { format } })),
  policies: () => execute(() => client.GET('/policies')),
  createPolicy: (body: Record<string, unknown>) => execute(() => client.POST('/policies', { body: body as never })),
  updatePolicy: (id: string, body: Record<string, unknown>) =>
    execute(() => client.PUT('/policies/{id}', { params: { path: { id } }, body: body as never })),
  deletePolicy: (id: string) => execute(() => client.DELETE('/policies/{id}', { params: { path: { id } } })),
  scopes: () => execute(() => client.GET('/scopes')),
  createScope: (body: Record<string, unknown>) => execute(() => client.POST('/scopes', { body: body as never })),
  updateScope: (id: string, body: Record<string, unknown>) =>
    execute(() => client.PATCH('/scopes/{id}', { params: { path: { id } }, body: body as never })),
  deleteScope: (id: string) => execute(() => client.DELETE('/scopes/{id}', { params: { path: { id } } })),
  monitors: () => execute(() => client.GET('/monitors')),
  createMonitor: (body: Record<string, unknown>) => execute(() => client.POST('/monitors', { body: body as never })),
  updateMonitor: (id: string, body: Record<string, unknown>) =>
    execute(() => client.PATCH('/monitors/{id}', { params: { path: { id } }, body: body as never })),
  monitorAction: (id: string, action: 'pause' | 'resume' | 'run') =>
    execute(() =>
      action === 'pause'
        ? client.POST('/monitors/{id}/pause', { params: { path: { id } } as never, body: {} as never })
        : action === 'resume'
          ? client.POST('/monitors/{id}/resume', { params: { path: { id } } as never, body: {} as never })
          : client.POST('/monitors/{id}/run', { params: { path: { id } } as never, body: {} as never }),
    ),
  deleteMonitor: (id: string) => execute(() => client.DELETE('/monitors/{id}', { params: { path: { id } } })),
  schedules: () => execute(() => client.GET('/schedules')),
  createSchedule: (body: Record<string, unknown>) => execute(() => client.POST('/schedules', { body: body as never })),
  updateSchedule: (id: string, body: Record<string, unknown>) =>
    execute(() => client.PATCH('/schedules/{id}', { params: { path: { id } }, body: body as never })),
  deleteSchedule: (id: string) => execute(() => client.DELETE('/schedules/{id}', { params: { path: { id } } })),
  githubTasks: () => execute(() => client.GET('/github/tasks')),
  createGitHubTask: (keyword: string) => execute(() => client.POST('/github/tasks', { body: { keyword } })),
  githubMonitors: () => execute(() => client.GET('/github/monitors')),
  createGitHubMonitor: (body: Record<string, unknown>) =>
    execute(() => client.POST('/github/monitors', { body: body as never })),
  updateGitHubMonitor: (id: string, body: Record<string, unknown>) =>
    execute(() => client.PATCH('/github/monitors/{id}', { params: { path: { id } }, body: body as never })),
  deleteGitHubMonitor: (id: string) =>
    execute(() => client.DELETE('/github/monitors/{id}', { params: { path: { id } } })),
  githubFindings: () => execute(() => client.GET('/github/findings')),
  fingerprints: () => execute(() => client.GET('/fingerprints')),
  createFingerprint: (body: Record<string, unknown>) =>
    execute(() => client.POST('/fingerprints', { body: body as never })),
  updateFingerprint: (id: string, body: Record<string, unknown>) =>
    execute(() => client.PATCH('/fingerprints/{id}', { params: { path: { id } }, body: body as never })),
  deleteFingerprint: (id: string) => execute(() => client.DELETE('/fingerprints/{id}', { params: { path: { id } } })),
  importFingerprints: async (content: string) => {
    const response = await fetch('/api/v1/fingerprints/import', {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/x-yaml', 'X-CSRF-Token': csrfFromCookie() },
      body: content,
    })
    if (!response.ok) throw new Error((await response.json().catch(() => ({}))).detail || '导入失败')
    return response.json()
  },
  pocPlugins: () => execute(() => client.GET('/poc/plugins')),
  syncPOC: () => execute(() => client.POST('/poc/sync', { body: {} as never })),
  users: () => execute(() => client.GET('/users')),
  user: (id: string) => execute(() => client.GET('/users/{id}', { params: { path: { id } } })),
  createUser: (body: { username: string; password: string; role: string }) =>
    execute(() => client.POST('/users', { body: body as never })),
  updateUser: (id: string, body: Record<string, unknown>) =>
    execute(() => client.PATCH('/users/{id}', { params: { path: { id } }, body: body as never })),
  deleteUser: (id: string) => execute(() => client.DELETE('/users/{id}', { params: { path: { id } } })),
  apiKeys: () => execute(() => client.GET('/api-keys')),
  createAPIKey: (body: { name: string; scopes: string[] }) =>
    execute(() => client.POST('/api-keys', { body: body as never })),
  deleteAPIKey: (id: string) => execute(() => client.DELETE('/api-keys/{id}', { params: { path: { id } } })),
  audit: (query?: PageQuery) => execute(() => client.GET('/audit-events', { params: { query: query as never } })),
  listRecycleBin: (query: { type?: string; q?: string; page?: number; pageSize?: number }) =>
    execute(() => client.GET('/recycle-bin', { params: { query: query as never } })),
  restoreRecord: (type: string, id: string) =>
    execute(() => client.POST('/recycle-bin/{type}/{id}/restore', { params: { path: { type, id } } as never })),
  systemInfo: () => execute(() => client.GET('/system/info')),
  ready: () => fetch('/readyz').then((response) => response.ok),
}

function csrfFromCookie(): string {
  const match = document.cookie.match(/(?:^|; )arl_csrf=([^;]+)/)
  return match ? decodeURIComponent(match[1]) : ''
}
