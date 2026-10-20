import { render, screen } from '@testing-library/react'
import { DataTable } from './data-table'

type Row = { id: string; name: string; severity: string }

describe('DataTable', () => {
  it('renders rows in desktop and mobile presentations', () => {
    render(
      <DataTable<Row>
        rows={[{ id: '1', name: 'example.com', severity: 'high' }]}
        rowKey={(row) => row.id}
        empty={<div>empty</div>}
        columns={[
          { key: 'name', header: '资产', cell: (row) => row.name },
          { key: 'severity', header: '风险', cell: (row) => row.severity },
        ]}
      />,
    )
    expect(screen.getAllByText('example.com')).toHaveLength(2)
    expect(screen.getAllByRole('columnheader', { name: '资产' })).toHaveLength(1)
  })

  it('renders the empty state when no rows exist', () => {
    render(
      <DataTable<Row>
        rows={[]}
        rowKey={(row) => row.id}
        empty={<div>暂无数据</div>}
        columns={[{ key: 'name', header: '资产', cell: (row) => row.name }]}
      />,
    )
    expect(screen.getByText('暂无数据')).toBeVisible()
  })
})
