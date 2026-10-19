// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest'
import Antd from 'ant-design-vue'
import EasmOrganizationsPage from './EasmOrganizationsPage.vue'

const apiMock = vi.fn()
const push = vi.fn()
vi.mock('@/api', () => ({
  api: (path: string, init?: unknown) => apiMock(path, init),
  qs: (params: Record<string, unknown>) => {
    const s = new URLSearchParams()
    Object.entries(params).forEach(([k, v]) => { if (v !== undefined && v !== null && v !== '') s.set(k, String(v)) })
    const q = s.toString()
    return q ? `?${q}` : ''
  },
}))
vi.mock('vue-router', () => ({
  useRouter: () => ({ push }),
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

describe('EasmOrganizationsPage', () => {
  afterEach(() => {
    apiMock.mockReset()
    push.mockReset()
  })

  it('lists organizations and opens detail', async () => {
    apiMock.mockResolvedValue({ data: { items: [{ id: 'o1', canonicalName: '单位A', region: '蜀', status: 'active' }] } })
    const wrapper = mount(EasmOrganizationsPage, { global: { plugins: [Antd] } })
    await flushPromises()
    expect(wrapper.text()).toContain('单位A')
    await wrapper.find('a.org-name').trigger('click')
    expect(push).toHaveBeenCalledWith('/easm/organizations/o1')
    wrapper.unmount()
  })

  it('shows import counts after upload', async () => {
    apiMock.mockImplementation((path: string) => {
      if (path === '/organizations/import') return Promise.resolve({ data: { created: 2, updated: 1, domainsQueued: 3, errors: [] } })
      return Promise.resolve({ data: { items: [] } })
    })
    const wrapper = mount(EasmOrganizationsPage, { attachTo: document.body, global: { plugins: [Antd] } })
    await flushPromises()
    const buttons = wrapper.findAll('button')
    const importButton = buttons.find((b) => b.text().includes('导入 CSV'))
    expect(importButton).toBeTruthy()
    await importButton!.trigger('click')
    await flushPromises()
    expect(document.body.textContent).toContain('canonicalName')
    wrapper.unmount()
  })
})
