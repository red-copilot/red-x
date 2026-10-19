// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { beforeAll, describe, expect, it, vi } from 'vitest'
import Antd from 'ant-design-vue'
import EasmAssetsPage from './EasmAssetsPage.vue'
import EasmFindingsPage from './EasmFindingsPage.vue'
import EasmTasksPage from './EasmTasksPage.vue'

const apiMock = vi.fn()
vi.mock('@/api', () => ({
  api: (path: string, init?: unknown) => apiMock(path, init),
  qs: (params: Record<string, unknown>) => {
    const s = new URLSearchParams()
    Object.entries(params).forEach(([k, v]) => { if (v !== undefined && v !== null && v !== '') s.set(k, String(v)) })
    const q = s.toString()
    return q ? `?${q}` : ''
  },
  formatTime: (value?: string) => value || '-',
}))
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
  useRoute: () => ({ query: {} }),
}))

beforeAll(() => {
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: false, media: query, onchange: null,
      addListener: vi.fn(), removeListener: vi.fn(), addEventListener: vi.fn(), removeEventListener: vi.fn(), dispatchEvent: vi.fn(),
    })),
  })
})

function response(path: string) {
  if (path.startsWith('/organizations')) return Promise.resolve({ data: { items: [{ id: 'o1', canonicalName: '单位A' }] } })
  if (path.startsWith('/easm/assets')) return Promise.resolve({ data: { items: [{ id: 'a1', kind: 'domain', key: 'a.edu.cn', risk: 'high', organizationIds: ['o1'], rootDomains: ['a.edu.cn'], firstSeen: '', lastSeen: '' }], total: 1, page: 1, pageSize: 20, truncated: false, unlocatedAssets: 0 } })
  if (path.startsWith('/easm/findings')) return Promise.resolve({ data: { items: [{ finding: { id: 'f1', kind: 'nuclei', title: '测试漏洞', target: 'a.edu.cn', severity: 'high', dedupeKey: 'x', createdAt: '' }, occurrences: 2, organizationIds: ['o1'], rootDomains: ['a.edu.cn'] }], total: 1, page: 1, pageSize: 20, truncated: false, unlinkedFindings: 0 } })
  if (path.startsWith('/easm/tasks')) return Promise.resolve({ data: { items: [{ id: 't1', name: 'EASM 单位A a.edu.cn', rootDomain: 'a.edu.cn', status: 'succeeded', progress: 100, createdAt: '' }], total: 1, page: 1, pageSize: 20 } })
  return Promise.resolve({ data: {} })
}

describe('EASM catalog pages', () => {
  it('renders assets', async () => {
    apiMock.mockImplementation(response)
    const wrapper = mount(EasmAssetsPage, { global: { plugins: [Antd] } })
    await flushPromises()
    expect(wrapper.text()).toContain('a.edu.cn')
    wrapper.unmount()
  })
  it('renders current findings', async () => {
    apiMock.mockImplementation(response)
    const wrapper = mount(EasmFindingsPage, { global: { plugins: [Antd] } })
    await flushPromises()
    expect(wrapper.text()).toContain('测试漏洞')
    expect(wrapper.text()).toContain('2')
    wrapper.unmount()
  })
  it('renders EASM tasks', async () => {
    apiMock.mockImplementation(response)
    const wrapper = mount(EasmTasksPage, { global: { plugins: [Antd] } })
    await flushPromises()
    expect(wrapper.text()).toContain('EASM 单位A a.edu.cn')
    wrapper.unmount()
  })
})
