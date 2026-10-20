import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { RotateCcw, Search, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api'
import { DataTable } from '@/components/common/data-table'
import { PageHeader } from '@/components/common/page-header'
import { Pagination } from '@/components/common/pagination'
import { EmptyState, ErrorState, LoadingBlock } from '@/components/common/states'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { formatTime } from '@/lib/format'

type DeletedRecord = {
  type: string
  id: string
  label: string
  deletedAt: string
  deletedBy?: string
}

const resourceLabels: Record<string, string> = {
  organization: '组织',
  organization_domain: '归属域名',
  asset: '资产',
  policy: '策略',
  scope: '资产组',
  monitor: '监控',
  schedule: '计划',
  fingerprint: '指纹',
  github_monitor: 'GitHub 监控',
  user: '用户',
}

export function RecycleBinPage() {
  const queryClient = useQueryClient()
  const [type, setType] = useState('')
  const [q, setQ] = useState('')
  const [page, setPage] = useState(1)
  const deleted = useQuery({
    queryKey: ['recycle-bin', type, q, page],
    queryFn: () => api.listRecycleBin({ type, q, page, pageSize: 20 }),
  })
  const restore = useMutation({
    mutationFn: ({ type: resourceType, id }: { type: string; id: string }) =>
      api.restoreRecord(resourceType, id),
    onSuccess: (_, item) => {
      toast.success(`${resourceLabels[item.type] || item.type}已恢复`)
      void queryClient.invalidateQueries({ queryKey: ['recycle-bin'] })
      void queryClient.invalidateQueries()
    },
    onError: (error) => toast.error(error.message),
  })
  const rows = (deleted.data?.items || []) as DeletedRecord[]

  return (
    <div>
      <PageHeader
        title="回收站"
        description="集中恢复已软删除的组织、资产、自动化配置和用户。恢复仅限管理员操作。"
      />
      <div className="mb-3 flex flex-wrap items-center gap-2">
        <div className="relative min-w-0 flex-1 sm:max-w-sm">
          <Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={q}
            onChange={(event) => {
              setQ(event.target.value)
              setPage(1)
            }}
            placeholder="搜索已删除记录…"
            className="pl-8"
          />
        </div>
        <Select
          value={type || 'all'}
          onValueChange={(value) => {
            setType(value === 'all' ? '' : value)
            setPage(1)
          }}
        >
          <SelectTrigger className="w-40">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">全部类型</SelectItem>
            {Object.entries(resourceLabels).map(([value, label]) => (
              <SelectItem key={value} value={value}>
                {label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      {deleted.isLoading ? (
        <LoadingBlock />
      ) : deleted.error ? (
        <ErrorState error={deleted.error} retry={() => deleted.refetch()} />
      ) : (
        <>
          <DataTable
            rows={rows}
            rowKey={(row) => `${row.type}-${row.id}`}
            empty={
              <EmptyState
                title="回收站为空"
                action={<Trash2 className="mx-auto mt-2 size-5 text-muted-foreground" />}
              />
            }
            columns={[
              { key: 'type', header: '类型', cell: (row) => resourceLabels[row.type] || row.type },
              { key: 'label', header: '记录', cell: (row) => <span className="font-medium">{row.label}</span> },
              { key: 'actor', header: '删除人', cell: (row) => row.deletedBy || '—', hideOnTablet: true },
              { key: 'time', header: '删除时间', cell: (row) => formatTime(row.deletedAt) },
              {
                key: 'actions',
                header: '操作',
                cell: (row) => (
                  <Button
                    size="sm"
                    variant="outline"
                    disabled={restore.isPending}
                    onClick={() => restore.mutate({ type: row.type, id: row.id })}
                  >
                    <RotateCcw />
                    恢复
                  </Button>
                ),
              },
            ]}
          />
          <Pagination
            page={deleted.data?.page || page}
            pageSize={deleted.data?.pageSize || 20}
            total={deleted.data?.total || 0}
            onChange={setPage}
          />
        </>
      )}
    </div>
  )
}
