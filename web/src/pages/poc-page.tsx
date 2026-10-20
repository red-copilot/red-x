import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Radar, RefreshCw } from 'lucide-react'
import { useMemo, useState } from 'react'
import { toast } from 'sonner'
import { api, type PocPlugin } from '@/api'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { DataTable } from '@/components/common/data-table'
import { PageHeader } from '@/components/common/page-header'
import { EmptyState, ErrorState, LoadingBlock } from '@/components/common/states'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { isAdmin } from '@/lib/access'
import { useAuth } from '@/app/auth'

export function PocPage() {
  const { role } = useAuth()
  const queryClient = useQueryClient()
  const [search, setSearch] = useState('')
  const [kind, setKind] = useState('all')
  const plugins = useQuery({ queryKey: ['poc-plugins'], queryFn: () => api.pocPlugins() })
  const sync = useMutation({
    mutationFn: () => api.syncPOC(),
    onSuccess: () => {
      toast.success('PoC 元数据已同步')
      void queryClient.invalidateQueries({ queryKey: ['poc-plugins'] })
    },
    onError: (error) => toast.error(error.message),
  })
  const rows = useMemo(
    () =>
      ((plugins.data?.items || []) as PocPlugin[]).filter(
        (row) =>
          (kind === 'all' || row.type === kind) &&
          (!search || `${row.name} ${row.appName} ${row.vulName}`.toLowerCase().includes(search.toLowerCase())),
      ),
    [plugins.data, kind, search],
  )
  return (
    <div>
      <PageHeader
        title="PoC 插件"
        description="查看 Go 原生 PoC、协议识别与弱口令能力元数据。"
        actions={
          isAdmin(role) ? (
            <Button onClick={() => sync.mutate()} disabled={sync.isPending}>
              <RefreshCw />
              同步元数据
            </Button>
          ) : null
        }
      />
      <div className="mb-3 flex flex-wrap gap-2">
        <Input
          className="max-w-sm"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="搜索插件、应用或漏洞…"
        />
        <Select value={kind} onValueChange={setKind}>
          <SelectTrigger className="w-36">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">全部类型</SelectItem>
            <SelectItem value="poc">PoC</SelectItem>
            <SelectItem value="sniffer">协议识别</SelectItem>
            <SelectItem value="brute">弱口令</SelectItem>
          </SelectContent>
        </Select>
      </div>
      {plugins.isLoading ? (
        <LoadingBlock />
      ) : plugins.error ? (
        <ErrorState error={plugins.error} retry={() => plugins.refetch()} />
      ) : (
        <DataTable
          rows={rows}
          rowKey={(row) => row.name || ''}
          empty={
            <EmptyState
              title="暂无插件元数据"
              action={<Radar className="mx-auto mt-2 size-5 text-muted-foreground" />}
            />
          }
          columns={[
            { key: 'name', header: '插件名', cell: (row) => <span className="mono font-medium">{row.name}</span> },
            {
              key: 'type',
              header: '类型',
              cell: (row) => (
                <Badge variant={row.type === 'poc' ? 'info' : row.type === 'brute' ? 'danger' : 'success'}>
                  {row.type}
                </Badge>
              ),
            },
            { key: 'app', header: '应用', cell: (row) => row.appName || '—', hideOnTablet: true },
            { key: 'vuln', header: '漏洞 / 能力', cell: (row) => row.vulName || '—' },
            { key: 'schemes', header: '协议', cell: (row) => row.schemes?.join(', ') || '—', hideOnTablet: true },
          ]}
        />
      )}
    </div>
  )
}
