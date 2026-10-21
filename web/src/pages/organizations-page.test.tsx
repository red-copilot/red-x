import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { api } from '@/api'
import { OrganizationsPage } from './organizations-page'

vi.mock('@/app/auth', () => ({
  useAuth: () => ({ role: 'operator' }),
}))

describe('OrganizationsPage', () => {
  it('looks up a domain and opens the matched organization for confirmation', async () => {
    vi.spyOn(api, 'organizations').mockResolvedValue({ items: [] } as never)
    vi.spyOn(api, 'map').mockResolvedValue({
      organizations: [],
      points: [],
      unlocatedAssets: 0,
      unlocatedOrganizations: 0,
      unlinkedFindings: 0,
      truncated: false,
      updatedAt: '2026-10-21T08:00:00Z',
    } as never)
    const lookup = vi.spyOn(api, 'lookupDomain').mockResolvedValue({
      organization: {
        id: 'org-1',
        canonicalName: '四川吉利学院',
        region: '蜀',
        status: 'active',
        createdAt: '2026-10-21T08:00:00Z',
        updatedAt: '2026-10-21T08:00:00Z',
      },
      attribution: {
        orgName: '四川吉利学院',
        icpNo: '蜀ICP备00000000号',
        auditDate: '2026-10-20',
      },
      confidence: 'high',
      provider: 'chinaz',
    } as never)
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })

    render(
      <QueryClientProvider client={client}>
        <MemoryRouter initialEntries={['/organizations']}>
          <Routes>
            <Route path="/organizations" element={<OrganizationsPage />} />
            <Route path="/organizations/:id" element={<div>组织确认工作区</div>} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    )

    fireEvent.click(await screen.findByRole('button', { name: '域名反查' }))
    fireEvent.change(screen.getByPlaceholderText('输入已授权主域，例如 guc.edu.cn'), {
      target: { value: 'guc.edu.cn' },
    })
    fireEvent.click(screen.getByRole('button', { name: '查询归属' }))

    await waitFor(() => expect(lookup).toHaveBeenCalledWith('guc.edu.cn'))
    expect(await screen.findByRole('dialog')).toHaveTextContent('四川吉利学院')
    expect(screen.getByText('蜀ICP备00000000号')).toBeVisible()
    expect(screen.getByText('高置信度')).toBeVisible()

    fireEvent.click(screen.getByRole('button', { name: '进入组织并确认归属' }))
    expect(await screen.findByText('组织确认工作区')).toBeVisible()
  })
})
