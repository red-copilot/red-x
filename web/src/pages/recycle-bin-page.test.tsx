import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { api } from '@/api'
import { RecycleBinPage } from './recycle-bin-page'

describe('RecycleBinPage', () => {
  it('lists deleted records and restores the selected record', async () => {
    const list = vi.spyOn(api, 'listRecycleBin').mockResolvedValue({
      items: [
        {
          type: 'policy',
          id: 'p1',
          label: '归档策略',
          deletedAt: '2026-10-20T08:00:00Z',
          deletedBy: 'operator',
        },
      ],
      total: 1,
      page: 1,
      pageSize: 20,
    })
    const restore = vi.spyOn(api, 'restoreRecord').mockResolvedValue({
      type: 'policy',
      id: 'p1',
      restoredAt: '2026-10-20T09:00:00Z',
    })
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })

    render(
      <QueryClientProvider client={client}>
        <RecycleBinPage />
      </QueryClientProvider>,
    )

    expect((await screen.findAllByText('归档策略')).length).toBeGreaterThan(0)
    expect(list).toHaveBeenCalledWith({ type: '', q: '', page: 1, pageSize: 20 })
    fireEvent.click(screen.getAllByRole('button', { name: '恢复' })[0])
    await waitFor(() => expect(restore).toHaveBeenCalledWith('policy', 'p1'))
  })
})
