import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'

export type Column<T> = {
  key: string
  header: string
  className?: string
  cell: (row: T) => ReactNode
  hideOnTablet?: boolean
  hideOnMobile?: boolean
}

export function DataTable<T>({
  rows,
  columns,
  rowKey,
  empty,
  loading,
  scrollable = false,
}: {
  rows: T[]
  columns: Column<T>[]
  rowKey: (row: T) => string
  empty: ReactNode
  loading?: boolean
  scrollable?: boolean
}) {
  if (loading)
    return <div className="rounded-lg border bg-card p-8 text-center text-xs text-muted-foreground">正在加载…</div>
  if (!rows.length) return <>{empty}</>
  return (
    <>
      <div className={cn('hidden overflow-hidden rounded-lg border bg-card md:block', scrollable && 'max-h-[calc(100vh-18rem)] overflow-y-auto scrollbar-thin')}>
        <div className="overflow-x-auto">
          <table className="w-full min-w-[720px] text-left text-xs">
            <thead className="sticky top-0 z-10 border-b bg-[#fafbfc] text-[11px] uppercase tracking-[0.06em] text-muted-foreground">
              <tr>
                {columns.map((column) => (
                  <th
                    key={column.key}
                    className={cn(
                      'px-3 py-2.5 font-medium',
                      column.className,
                      column.hideOnTablet && 'hidden xl:table-cell',
                    )}
                  >
                    {column.header}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody className="divide-y">
              {rows.map((row) => (
                <tr key={rowKey(row)} className="transition-colors hover:bg-[#fafbfc]">
                  {columns.map((column) => (
                    <td
                      key={column.key}
                      className={cn(
                        'px-3 py-3 align-middle',
                        column.className,
                        column.hideOnTablet && 'hidden xl:table-cell',
                      )}
                    >
                      {column.cell(row)}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
      <div className="space-y-2 md:hidden">
        {rows.map((row) => (
          <div key={rowKey(row)} className="rounded-lg border bg-card p-3">
            <dl className="space-y-2">
              {columns
                .filter((column) => !column.hideOnMobile)
                .map((column) => (
                  <div key={column.key} className="flex items-start justify-between gap-3 text-xs">
                    <dt className="shrink-0 text-muted-foreground">{column.header}</dt>
                    <dd className="min-w-0 text-right">{column.cell(row)}</dd>
                  </div>
                ))}
            </dl>
          </div>
        ))}
      </div>
    </>
  )
}
