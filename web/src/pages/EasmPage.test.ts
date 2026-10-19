// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import EasmPage from './EasmPage.vue'

const apiMock = vi.fn()
vi.mock('@/api', () => ({
  api: (path: string, init?: unknown) => apiMock(path, init),
  qs: (params: Record<string, unknown>) => {
    const search = new URLSearchParams()
    Object.entries(params).forEach(([key, value]) => {
      if (value !== undefined && value !== null && value !== '') search.set(key, String(value))
    })
    const text = search.toString()
    return text ? `?${text}` : ''
  },
}))

function baseResponses() {
  return (path: string) => {
    if (path === '/organizations') {
      return Promise.resolve({ data: { items: [{ id: 'o1', canonicalName: '单位A', status: 'active' }] } })
    }
    if (path === '/easm/health') {
      return Promise.resolve({ data: { chinaz: 'unavailable', icpQuery: 'ok', geoip: 'ok', neo4j: 'ok' } })
    }
    if (path.startsWith('/easm/map')) {
      return Promise.resolve({ data: {
        organizations: [{ orgId: 'o1', name: '单位A', assetCount: 3, risk: 'high', lat: 30, lon: 104 }],
        points: [], unlocatedAssets: 3, unlocatedOrganizations: 0, unlinkedFindings: 0, updatedAt: '',
      } })
    }
    if (path === '/organizations/o1/domains') return Promise.resolve({ data: { items: [] } })
    if (path.startsWith('/easm/graph')) {
      return Promise.resolve({ data: { nodes: [], edges: [], truncated: false, aggregated: false } })
    }
    return Promise.resolve({ data: {} })
  }
}

describe('EasmPage', () => {
  afterEach(() => {
    apiMock.mockReset()
    vi.restoreAllMocks()
  })

  it('renders organization rail and selects the first org', async () => {
    apiMock.mockImplementation(baseResponses())
    const wrapper = mount(EasmPage, { global: { stubs: { WorldMap: { template: '<div class="map-stub" />' } } } })
    await flushPromises()
    expect(wrapper.text()).toContain('单位A')
    expect(wrapper.text()).toContain('EASM')
    expect(apiMock).toHaveBeenCalledWith(expect.stringContaining('/easm/map'), undefined)
    wrapper.unmount()
  })

  it('shows the ICP error banner when lookup fails', async () => {
    apiMock.mockImplementation((path: string) => {
      if (path === '/icp/lookup') return Promise.reject(new Error('ICP query credential is invalid'))
      return baseResponses()(path)
    })
    const wrapper = mount(EasmPage, { global: { stubs: { WorldMap: { template: '<div class="map-stub" />' } } } })
    await flushPromises()
    await wrapper.find('input[placeholder="输入主域，反查备案主体"]').setValue('guc.edu.cn')
    await wrapper.find('form.search').trigger('submit')
    await flushPromises()
    expect(wrapper.text()).toContain('ICP query credential is invalid')
    wrapper.unmount()
  })

  it('toggles the graph layer and shows unlocated counts', async () => {
    apiMock.mockImplementation(baseResponses())
    const wrapper = mount(EasmPage, { global: { stubs: { WorldMap: { template: '<div class="map-stub" />' } } } })
    await flushPromises()
    expect(wrapper.text()).toContain('未定位资产 3')
    const graphButton = wrapper.findAll('button').find((b) => b.text() === '图谱')
    expect(graphButton).toBeTruthy()
    await graphButton!.trigger('click')
    await flushPromises()
    expect(apiMock).toHaveBeenCalledWith(expect.stringContaining('/easm/graph'), undefined)
    wrapper.unmount()
  })
})
