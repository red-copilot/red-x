import { useQuery } from '@tanstack/react-query'
import { Crosshair, Search } from 'lucide-react'
import { useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { api, type EasmAsset } from '@/api'
import { RiskBadge } from '@/components/common/badges'
import { DataTable } from '@/components/common/data-table'
import { PageHeader } from '@/components/common/page-header'
import { Pagination } from '@/components/common/pagination'
import { EmptyState, ErrorState, LoadingBlock } from '@/components/common/states'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { formatTime } from '@/lib/format'

const surfaceKinds = ['', 'domain', 'ip', 'service']
const inventoryKinds = ['', 'domain', 'ip', 'site', 'url', 'cert', 'service', 'cidr']

export function AssetsPage() {
  const [params, setParams] = useSearchParams()
  const [selected, setSelected] = useState<EasmAsset | null>(null)
  const view = params.get('view') === 'inventory' ? 'inventory' : 'surface'
  const kind = params.get('kind') || ''
  const risk = params.get('risk') || ''
  const q = params.get('q') || ''
  const page = Number(params.get('page') || 1)
  const focus = params.get('focus') || ''
  const query = useQuery({
    queryKey: ['assets', view, kind, risk, q, page],
    queryFn: () => api.assets({ view, kind, risk, q, page, pageSize: 20 }),
    refetchInterval: 30_000,
  })
  const detail = useQuery({
    queryKey: ['asset', view, focus],
    queryFn: () => api.asset(focus, view),
    enabled: Boolean(focus),
  })

  function setParam(key: string, value: string) {
    const next = new URLSearchParams(params)
    if (value) next.set(key, value)
    else next.delete(key)
    if (key !== 'page') next.delete('page')
    setParams(next, { replace: true })
  }
  function open(row: EasmAsset) {
    setSelected(row)
    setParam('focus', row.id || '')
  }
  const visible = selected || detail.data
  const kinds = view === 'surface' ? surfaceKinds : inventoryKinds

  return (
    <div>
      <PageHeader
        title="攻击面资产"
        description="默认展示能从已确认组织主域关联到的域名、IP 与服务；原始扫描库存作为辅助视图保留。"
      />
      <div className="mb-3 flex flex-wrap items-center gap-2">
        <div className="flex rounded-md bg-muted p-1">
          <Button
            size="sm"
            variant={view === 'surface' ? 'default' : 'ghost'}
            onClick={() => setParam('view', 'surface')}
          >
            暴露面
          </Button>
          <Button
            size="sm"
            variant={view === 'inventory' ? 'default' : 'ghost'}
            onClick={() => setParam('view', 'inventory')}
          >
            原始库存
          </Button>
        </div>
        <form
          className="relative min-w-0 flex-1 sm:max-w-sm"
          onSubmit={(event) => {
            event.preventDefault()
            setParam('q', q)
          }}
        >
          <Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={params.get('q') || ''}
            onChange={(event) => setParam('q', event.target.value)}
            placeholder="搜索资产 key 或标题…"
            className="pl-8"
          />
        </form>
        <Select value={kind || 'all'} onValueChange={(value) => setParam('kind', value === 'all' ? '' : value)}>
          <SelectTrigger className="w-32">
            <SelectValue placeholder="全部类型" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">全部类型</SelectItem>
            {kinds.filter(Boolean).map((item) => (
              <SelectItem key={item} value={item}>
                {item}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {view === 'surface' ? (
          <Select value={risk || 'all'} onValueChange={(value) => setParam('risk', value === 'all' ? '' : value)}>
            <SelectTrigger className="w-32">
              <SelectValue placeholder="全部风险" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">全部风险</SelectItem>
              <SelectItem value="critical">严重</SelectItem>
              <SelectItem value="high">高危</SelectItem>
              <SelectItem value="medium">中危</SelectItem>
              <SelectItem value="low">低危</SelectItem>
            </SelectContent>
          </Select>
        ) : null}
      </div>
      {query.isLoading ? (
        <LoadingBlock />
      ) : query.error ? (
        <ErrorState error={query.error} retry={() => query.refetch()} />
      ) : (
        <>
          <DataTable
            rows={query.data?.items || []}
            rowKey={(row) => `${row.kind}-${row.id || row.key}`}
            empty={
              <EmptyState
                title="没有匹配资产"
                description="调整筛选条件，或先为组织确认主域并发起扫描。"
                action={<Crosshair className="mx-auto mt-2 size-5 text-muted-foreground" />}
              />
            }
            columns={[
              {
                key: 'key',
                header: '资产',
                cell: (row) => (
                  <button
                    className="mono max-w-[360px] truncate text-left font-medium text-primary hover:underline"
                    onClick={() => open(row)}
                  >
                    {row.key}
                  </button>
                ),
              },
              { key: 'kind', header: '类型', cell: (row) => row.kind || '—' },
              { key: 'risk', header: '风险', cell: (row) => <RiskBadge risk={row.risk} /> },
              { key: 'title', header: '标题', cell: (row) => row.title || '—', hideOnTablet: true },
              {
                key: 'orgs',
                header: '组织',
                cell: (row) => (row.organizationIds?.length ? `${row.organizationIds.length} 个` : '—'),
                hideOnTablet: true,
              },
              { key: 'lastSeen', header: '最近发现', cell: (row) => formatTime(row.lastSeen), hideOnTablet: true },
            ]}
          />
          <Pagination
            page={query.data?.page || page}
            pageSize={query.data?.pageSize || 20}
            total={query.data?.total || 0}
            onChange={(next) => setParam('page', String(next))}
          />
        </>
      )}
      <Sheet open={Boolean(visible)} onOpenChange={(open) => !open && (setSelected(null), setParam('focus', ''))}>
        <SheetContent>
          <SheetHeader>
            <SheetTitle className="mono break-all">{visible?.key}</SheetTitle>
            <SheetDescription>
              {visible?.kind} · {visible?.title || '无标题'}
            </SheetDescription>
          </SheetHeader>
          {visible ? (
            <div className="space-y-4">
              <div className="grid grid-cols-2 gap-2 text-xs">
                <Fact label="风险" value={<RiskBadge risk={visible.risk} />} />
                <Fact label="类型" value={visible.kind || '—'} />
                <Fact label="首次发现" value={formatTime(visible.firstSeen)} />
                <Fact label="最近发现" value={formatTime(visible.lastSeen)} />
              </div>
              {visible.rootDomains?.length ? (
                <Section title="关联主域">
                  <div className="flex flex-wrap gap-1.5">
                    {visible.rootDomains.map((domain) => (
                      <span key={domain} className="mono rounded border bg-muted px-2 py-1 text-[11px]">
                        {domain}
                      </span>
                    ))}
                  </div>
                </Section>
              ) : null}
              {visible.organizationIds?.length ? (
                <Section title="所属组织">
                  <div className="flex flex-wrap gap-1.5">
                    {visible.organizationIds.map((id) => (
                      <span key={id} className="mono rounded border bg-muted px-2 py-1 text-[11px]">
                        {id}
                      </span>
                    ))}
                  </div>
                </Section>
              ) : null}
              <Section title="观测摘要">
                <KeyValues data={visible.data as Record<string, unknown> | undefined} />
              </Section>
            </div>
          ) : (
            <LoadingBlock />
          )}
        </SheetContent>
      </Sheet>
    </div>
  )
}

function Fact({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="rounded-md border bg-[#fbfcfe] p-2.5">
      <div className="text-[10px] text-muted-foreground">{label}</div>
      <div className="mt-1 text-xs font-medium">{value}</div>
    </div>
  )
}
function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div>
      <div className="mb-2 text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground">{title}</div>
      {children}
    </div>
  )
}
function KeyValues({ data }: { data?: Record<string, unknown> }) {
  const entries = data
    ? Object.entries(data)
        .filter(([, value]) => value !== null && value !== '' && typeof value !== 'object')
        .slice(0, 20)
    : []
  if (!entries.length) return <div className="text-xs text-muted-foreground">暂无结构化观测字段</div>
  return (
    <dl className="divide-y rounded-md border text-xs">
      {entries.map(([key, value]) => (
        <div key={key} className="grid grid-cols-[120px_1fr] gap-2 px-3 py-2">
          <dt className="truncate text-muted-foreground">{key}</dt>
          <dd className="break-all">{String(value)}</dd>
        </div>
      ))}
    </dl>
  )
}
