import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Crosshair, Pencil, Plus, Search, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { toast } from 'sonner'
import { api, type EasmAsset } from '@/api'
import { RiskBadge } from '@/components/common/badges'
import { DataTable } from '@/components/common/data-table'
import { PageHeader } from '@/components/common/page-header'
import { Pagination } from '@/components/common/pagination'
import { EmptyState, ErrorState, LoadingBlock } from '@/components/common/states'
import { ConfirmDialog } from '@/components/common/confirm-dialog'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { Textarea } from '@/components/ui/textarea'
import { formatTime } from '@/lib/format'
import { canWrite } from '@/lib/access'
import { useAuth } from '@/app/auth'

const surfaceKinds = ['', 'domain', 'ip', 'service']
const inventoryKinds = ['', 'domain', 'ip', 'site', 'url', 'cert', 'service', 'cidr']

export function AssetsPage() {
  const { role } = useAuth()
  const queryClient = useQueryClient()
  const [params, setParams] = useSearchParams()
  const [selected, setSelected] = useState<EasmAsset | null>(null)
  const [editorOpen, setEditorOpen] = useState(false)
  const [editingId, setEditingId] = useState('')
  const [form, setForm] = useState({
    kind: 'domain',
    key: '',
    displayName: '',
    displayUrl: '',
    notes: '',
    tags: '',
  })
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
  const create = useMutation({
    mutationFn: () =>
      api.createAsset({
        ...form,
        tags: form.tags
          .split(',')
          .map((item) => item.trim())
          .filter(Boolean),
      }),
    onSuccess: () => {
      toast.success('资产已创建')
      setEditorOpen(false)
      void queryClient.invalidateQueries({ queryKey: ['assets'] })
    },
    onError: (error) => toast.error(error.message),
  })
  const update = useMutation({
    mutationFn: () =>
      api.updateAsset(editingId, {
        displayName: form.displayName,
        displayUrl: form.displayUrl,
        notes: form.notes,
        tags: form.tags
          .split(',')
          .map((item) => item.trim())
          .filter(Boolean),
      }),
    onSuccess: () => {
      toast.success('资产信息已更新')
      setEditorOpen(false)
      setEditingId('')
      void queryClient.invalidateQueries({ queryKey: ['assets'] })
      void queryClient.invalidateQueries({ queryKey: ['asset'] })
    },
    onError: (error) => toast.error(error.message),
  })
  const remove = useMutation({
    mutationFn: (id: string) => api.deleteAsset(id),
    onSuccess: () => {
      toast.success('资产已移入回收站')
      setSelected(null)
      setParam('focus', '')
      void queryClient.invalidateQueries({ queryKey: ['assets'] })
      void queryClient.invalidateQueries({ queryKey: ['organization-assets'] })
    },
    onError: (error) => toast.error(error.message),
  })

  function openCreate() {
    setEditingId('')
    setForm({ kind: 'domain', key: '', displayName: '', displayUrl: '', notes: '', tags: '' })
    setEditorOpen(true)
  }
  function openEdit(row: EasmAsset) {
    setEditingId(row.id || '')
    setForm({
      kind: row.kind || 'domain',
      key: row.key || '',
      displayName: row.displayName || '',
      displayUrl: row.displayUrl || '',
      notes: row.notes || '',
      tags: (row.tags || []).join(', '),
    })
    setEditorOpen(true)
  }

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
        actions={
          canWrite(role) && view === 'inventory' ? (
            <Button onClick={openCreate}>
              <Plus />
              新建资产
            </Button>
          ) : null
        }
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
            scrollable
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
              {
                key: 'actions',
                header: '操作',
                cell: (row) =>
                  canWrite(role) ? (
                    <div className="flex justify-end gap-1">
                      <Button size="sm" variant="ghost" onClick={() => openEdit(row)}>
                        <Pencil />
                        编辑
                      </Button>
                      <ConfirmDialog
                        title="删除资产"
                        description="资产及其观测会从当前视图隐藏并移入回收站；后续扫描再次发现时会自动恢复。"
                        confirmLabel="移入回收站"
                        onConfirm={() => remove.mutate(row.id || '')}
                        trigger={
                          <Button size="sm" variant="ghost" className="text-destructive">
                            <Trash2 />
                            删除
                          </Button>
                        }
                      />
                    </div>
                  ) : null,
              },
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
              {canWrite(role) ? (
                <div className="flex flex-wrap justify-end gap-2">
                  <Button size="sm" variant="outline" onClick={() => openEdit(visible)}>
                    <Pencil />
                    编辑基础信息
                  </Button>
                  <ConfirmDialog
                    title="删除资产"
                    description="资产会移入回收站；后续扫描再次发现时会自动恢复。"
                    confirmLabel="移入回收站"
                    onConfirm={() => remove.mutate(visible.id || '')}
                    trigger={
                      <Button size="sm" variant="outline" className="text-destructive">
                        <Trash2 />
                        删除
                      </Button>
                    }
                  />
                </div>
              ) : null}
              {visible.restoreReason === 'scan_rediscovery' && visible.restoredAt ? (
                <div className="rounded-md border border-sky-200 bg-sky-50 px-3 py-2 text-xs text-sky-800">
                  扫描重新发现，已于 {formatTime(visible.restoredAt)} 自动恢复。
                </div>
              ) : null}
              <div className="grid grid-cols-2 gap-2 text-xs">
                <Fact label="风险" value={<RiskBadge risk={visible.risk} />} />
                <Fact label="类型" value={visible.kind || '—'} />
                <Fact label="首次发现" value={formatTime(visible.firstSeen)} />
                <Fact label="最近发现" value={formatTime(visible.lastSeen)} />
                <Fact label="来源" value={visible.origin === 'manual' ? '手工维护' : '扫描发现'} />
              </div>
              {visible.displayName || visible.displayUrl || visible.notes || visible.tags?.length ? (
                <Section title="维护信息">
                  <div className="space-y-2 rounded-md border bg-[#fbfcfe] p-3 text-xs">
                    {visible.displayName ? <div>名称：{visible.displayName}</div> : null}
                    {visible.displayUrl ? <div className="mono break-all">URL：{visible.displayUrl}</div> : null}
                    {visible.notes ? <div className="whitespace-pre-wrap">备注：{visible.notes}</div> : null}
                    {visible.tags?.length ? (
                      <div className="flex flex-wrap gap-1.5">
                        {visible.tags.map((tag) => (
                          <span key={tag} className="rounded border bg-muted px-2 py-0.5 text-[11px]">
                            {tag}
                          </span>
                        ))}
                      </div>
                    ) : null}
                  </div>
                </Section>
              ) : null}
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
      <Dialog open={editorOpen} onOpenChange={setEditorOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{editingId ? '编辑资产基础信息' : '新建手工资产'}</DialogTitle>
            <DialogDescription>
              {editingId ? '类型、唯一键和扫描原始数据保持只读。' : '手工资产会标记为 manual 来源，不修改扫描生成的原始 data。'}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-3">
            <div className="grid gap-3 sm:grid-cols-2">
              <div className="space-y-1.5">
                <Label>类型</Label>
                <Select
                  value={form.kind}
                  onValueChange={(value) => setForm({ ...form, kind: value })}
                  disabled={Boolean(editingId)}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {inventoryKinds
                      .filter(Boolean)
                      .map((item) => (
                        <SelectItem key={item} value={item}>
                          {item}
                        </SelectItem>
                      ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <Label>唯一键</Label>
                <Input
                  value={form.key}
                  disabled={Boolean(editingId)}
                  onChange={(event) => setForm({ ...form, key: event.target.value })}
                  placeholder="example.com / 1.1.1.1 / https://…"
                />
              </div>
            </div>
            <div className="space-y-1.5">
              <Label>显示名称</Label>
              <Input
                value={form.displayName}
                onChange={(event) => setForm({ ...form, displayName: event.target.value })}
              />
            </div>
            <div className="space-y-1.5">
              <Label>URL</Label>
              <Input
                value={form.displayUrl}
                onChange={(event) => setForm({ ...form, displayUrl: event.target.value })}
              />
            </div>
            <div className="space-y-1.5">
              <Label>标签（逗号分隔）</Label>
              <Input value={form.tags} onChange={(event) => setForm({ ...form, tags: event.target.value })} />
            </div>
            <div className="space-y-1.5">
              <Label>备注</Label>
              <Textarea value={form.notes} onChange={(event) => setForm({ ...form, notes: event.target.value })} />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setEditorOpen(false)}>
              取消
            </Button>
            <Button
              disabled={(!editingId && (!form.kind || !form.key.trim())) || create.isPending || update.isPending}
              onClick={() => (editingId ? update.mutate() : create.mutate())}
            >
              {(create.isPending || update.isPending) ? '保存中…' : editingId ? '保存修改' : '创建资产'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
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
