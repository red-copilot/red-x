// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest'
import Antd from 'ant-design-vue'
import EasmOrganizationDetailPage from './EasmOrganizationDetailPage.vue'

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
  useRoute: () => ({ params: { id: 'o1' }, query: {} }),
  useRouter: () => ({ push: vi.fn() }),
}))

function responses(path: string) {
  if (path === '/organizations/o1') return Promise.resolve({ data: { organization: { id: 'o1', canonicalName: '单位A', region: '蜀', status: 'active' }, domains: [{ id: 'd1', organizationId: 'o1', rootDomain: 'a.edu.cn', status: 'candidate', source: 'manual', confidence: 'high' }] } })
  if (path === '/easm/organizations/o1/summary') return Promise.resolve({ data: { organizationId: 'o1', confirmedDomains: 0, candidateDomains: 1, domainAssets: 0, ipAssets: 0, services: 0, currentFindings: 0, criticalFindings: 0, highFindings: 0, mediumFindings: 0, lowFindings: 0, infoFindings: 0, historyFindings: 0, unlocatedAssets: 0, linkedTasks: 0 } })
  if (path === '/organizations?limit=200') return Promise.resolve({ data: { items: [{ id: 'o1', canonicalName: '单位A' }, { id: 'o2', canonicalName: '单位B' }] } })
  if (path.startsWith('/easm/organizations/o1/tasks')) return Promise.resolve({ data: { items: [{ id: 't1', name: 'EASM 单位A a.edu.cn', rootDomain: 'a.edu.cn', status: 'succeeded', progress: 100, createdAt: '' }], total: 1, page: 1, pageSize: 20 } })
  return Promise.resolve({ data: { items: [], total: 0, page: 1, pageSize: 20 } })
}

beforeAll(() => {
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: false, media: query, onchange: null,
      addListener: vi.fn(), removeListener: vi.fn(), addEventListener: vi.fn(), removeEventListener: vi.fn(), dispatchEvent: vi.fn(),
    })),
  })
})

describe('EasmOrganizationDetailPage', () => {
  afterEach(() => apiMock.mockReset())

  it('renders summary and candidate domains', async () => {
    apiMock.mockImplementation(responses)
    const wrapper = mount(EasmOrganizationDetailPage, { global: { plugins: [Antd] } })
    await flushPromises()
    expect(wrapper.text()).toContain('单位A')
    expect(wrapper.text()).toContain('候选主域')
    await wrapper.findAll('.ant-tabs-tab-btn').find((b) => b.text().includes('域名治理'))!.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('a.edu.cn')
    expect(wrapper.text()).toContain('确认所选')
    wrapper.unmount()
  })
})
