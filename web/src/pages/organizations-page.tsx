import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { FileUp, Pencil, Plus, RefreshCw, Search, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import { api, type LookupResult, type Organization } from '@/api'
import { RiskBadge, StatusBadge } from '@/components/common/badges'
import { DataTable } from '@/components/common/data-table'
import { EmptyState, ErrorState, LoadingBlock } from '@/components/common/states'
import { PageHeader } from '@/components/common/page-header'
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
import { canWrite } from '@/lib/access'
import { useAuth } from '@/app/auth'

export function OrganizationsPage() {
  const { role } = useAuth()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [query, setQuery] = useState('')
  const [createOpen, setCreateOpen] = useState(false)
  const [importOpen, setImportOpen] = useState(false)
  const [editOpen, setEditOpen] = useState(false)
  const [lookupOpen, setLookupOpen] = useState(false)
  const [lookupDomain, setLookupDomain] = useState('')
  const [lookupResult, setLookupResult] = useState<LookupResult | null>(null)
  const [editingId, setEditingId] = useState('')
  const [form, setForm] = useState({ canonicalName: '', aliases: '', region: '', uscc: '' })
  const orgs = useQuery({ queryKey: ['organizations', query], queryFn: () => api.organizations(query) })
  const map = useQuery({ queryKey: ['map'], queryFn: () => api.map() })

  const create = useMutation({
    mutationFn: () =>
      api.createOrganization({
        canonicalName: form.canonicalName,
        aliases: form.aliases
          .split('|')
          .map((item) => item.trim())
          .filter(Boolean),
        region: form.region,
        uscc: form.uscc,
      }),
    onSuccess: () => {
      toast.success('组织已创建')
      setCreateOpen(false)
      setForm({ canonicalName: '', aliases: '', region: '', uscc: '' })
      void queryClient.invalidateQueries({ queryKey: ['organizations'] })
    },
    onError: (error) => toast.error(error.message),
  })
  const importOrgs = useMutation({
    mutationFn: (file: File) => api.importOrganizations(file),
    onSuccess: () => {
      toast.success('组织数据已导入')
      setImportOpen(false)
      void queryClient.invalidateQueries({ queryKey: ['organizations'] })
    },
    onError: (error) => toast.error(error.message),
  })
  const update = useMutation({
    mutationFn: () =>
      api.updateOrganization(editingId, {
        canonicalName: form.canonicalName,
        aliases: form.aliases
          .split('|')
          .map((item) => item.trim())
          .filter(Boolean),
        region: form.region,
        uscc: form.uscc,
      }),
    onSuccess: () => {
      toast.success('组织已更新')
      setEditOpen(false)
      setEditingId('')
      void queryClient.invalidateQueries({ queryKey: ['organizations'] })
      void queryClient.invalidateQueries({ queryKey: ['organization', editingId] })
      void queryClient.invalidateQueries({ queryKey: ['map'] })
    },
    onError: (error) => toast.error(error.message),
  })
  const lookup = useMutation({
    mutationFn: () => api.lookupDomain(lookupDomain.trim()),
    onSuccess: (result) => {
      setLookupResult(result)
      void queryClient.invalidateQueries({ queryKey: ['organizations'] })
      void queryClient.invalidateQueries({ queryKey: ['map'] })
    },
    onError: (error) => {
      setLookupResult(null)
      toast.error(error.message)
    },
  })
  const remove = useMutation({
    mutationFn: (id: string) => api.deleteOrganization(id),
    onSuccess: () => {
      toast.success('组织已移入回收站')
      void queryClient.invalidateQueries({ queryKey: ['organizations'] })
      void queryClient.invalidateQueries({ queryKey: ['map'] })
    },
    onError: (error) => toast.error(error.message),
  })

  function openDomainLookup() {
    setLookupDomain('')
    setLookupResult(null)
    setLookupOpen(true)
  }

  function editOrganization(org: Organization) {
    setEditingId(org.id || '')
    setForm({
      canonicalName: org.canonicalName || '',
      aliases: (org.aliases || []).join(' | '),
      region: org.region || '',
      uscc: org.uscc || '',
    })
    setEditOpen(true)
  }

  const riskByOrg = new Map((map.data?.organizations || []).map((item) => [item.orgId, item]))
  const rows = orgs.data?.items || []
  return (
    <div>
      <PageHeader
        title="组织治理"
        description="按单位维护备案主域、归属证据、风险覆盖与扫描任务。组织是 ARL 攻击面工作区的主线。"
        actions={
          canWrite(role) ? (
            <>
              <Button variant="outline" onClick={openDomainLookup}>
                <Search />
                域名反查
              </Button>
              <Button variant="outline" onClick={() => setImportOpen(true)}>
                <FileUp />
                导入 CSV
              </Button>
              <Button onClick={() => setCreateOpen(true)}>
                <Plus />
                新建组织
              </Button>
            </>
          ) : null
        }
      />
      <div className="mb-3 flex flex-wrap items-center gap-2">
        <form
          className="relative min-w-0 flex-1 sm:max-w-sm"
          onSubmit={(event) => {
            event.preventDefault()
            void orgs.refetch()
          }}
        >
          <Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="搜索组织名称、别名或信用代码…"
            className="pl-8"
          />
        </form>
        <Button variant="outline" onClick={() => void orgs.refetch()}>
          <RefreshCw />
          刷新
        </Button>
        <span className="ml-auto text-[11px] text-muted-foreground">共 {rows.length} 个组织</span>
      </div>
      {orgs.isLoading ? (
        <LoadingBlock />
      ) : orgs.error ? (
        <ErrorState error={orgs.error} retry={() => orgs.refetch()} />
      ) : (
        <DataTable
          rows={rows}
          rowKey={(row) => row.id || row.canonicalName || 'org'}
          empty={
            <EmptyState
              title="还没有组织"
              description="导入包含组织名称和主域名的 CSV，或先创建单个组织。"
              action={
                canWrite(role) ? (
                  <Button onClick={() => setImportOpen(true)}>
                    <FileUp />
                    导入组织
                  </Button>
                ) : undefined
              }
            />
          }
          columns={[
            {
              key: 'name',
              header: '组织',
              cell: (row) => (
                <Link to={`/organizations/${row.id}`} className="font-medium text-primary hover:underline">
                  {row.canonicalName}
                </Link>
              ),
            },
            { key: 'region', header: '地区', cell: (row) => row.region || '—', hideOnTablet: true },
            {
              key: 'aliases',
              header: '别名',
              cell: (row) =>
                row.aliases?.length ? (
                  <span className="text-muted-foreground">{row.aliases.slice(0, 2).join(' / ')}</span>
                ) : (
                  '—'
                ),
              hideOnTablet: true,
            },
            { key: 'risk', header: '最高风险', cell: (row) => <RiskBadge risk={riskByOrg.get(row.id || '')?.risk} /> },
            {
              key: 'assets',
              header: '暴露资产',
              cell: (row) => <span className="tabular-nums">{riskByOrg.get(row.id || '')?.assetCount ?? '—'}</span>,
            },
            { key: 'status', header: '状态', cell: (row) => <StatusBadge status={row.status} /> },
            {
              key: 'updated',
              header: '最近更新',
              cell: (row) => (row.updatedAt ? new Date(row.updatedAt).toLocaleDateString('zh-CN') : '—'),
              hideOnTablet: true,
            },
            {
              key: 'actions',
              header: '操作',
              cell: (row) =>
                canWrite(role) ? (
                  <div className="flex justify-end gap-1">
                    <Button size="sm" variant="ghost" onClick={() => editOrganization(row)}>
                      <Pencil />
                      编辑
                    </Button>
                    <ConfirmDialog
                      title="删除组织"
                      description="组织会移入回收站，已有域名和任务历史不会永久删除。存在排队或运行任务时无法删除。"
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
      )}

      <Dialog
        open={lookupOpen}
        onOpenChange={(open) => {
          setLookupOpen(open)
          if (!open) {
            setLookupDomain('')
            setLookupResult(null)
          }
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>域名反查组织归属</DialogTitle>
            <DialogDescription>
              通过备案数据识别主域所属单位。查询成功的域名会作为待确认归属写入组织，不会自动进入统计、地图或图谱。
            </DialogDescription>
          </DialogHeader>
          {lookupResult ? (
            <div className="space-y-4">
              <div className="rounded-lg border bg-muted/30 p-4">
                <div className="text-base font-semibold">{lookupResult.organization.canonicalName}</div>
                <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                  <span>{lookupResult.organization.region || '地区未知'}</span>
                  <span>·</span>
                  <span>{confidenceLabel(lookupResult.confidence)}</span>
                  <span>·</span>
                  <span>{providerLabel(lookupResult.provider)}</span>
                </div>
              </div>
              <dl className="grid gap-3 text-xs sm:grid-cols-2">
                <div>
                  <dt className="text-muted-foreground">备案主体</dt>
                  <dd className="mt-1 font-medium">{lookupResult.attribution.orgName || '—'}</dd>
                </div>
                <div>
                  <dt className="text-muted-foreground">备案号</dt>
                  <dd className="mono mt-1 font-medium">{lookupResult.attribution.icpNo || '—'}</dd>
                </div>
                <div>
                  <dt className="text-muted-foreground">审核时间</dt>
                  <dd className="mt-1 font-medium">{lookupResult.attribution.auditDate || '—'}</dd>
                </div>
                <div>
                  <dt className="text-muted-foreground">归属状态</dt>
                  <dd className="mt-1 font-medium">待人工确认</dd>
                </div>
              </dl>
              <p className="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-[11px] leading-5 text-amber-900">
                已写入候选归属。进入组织后请核对备案证据并确认，确认前不会纳入组织资产统计。
              </p>
              <DialogFooter>
                <Button
                  variant="outline"
                  onClick={() => {
                    setLookupResult(null)
                    setLookupDomain('')
                  }}
                >
                  重新查询
                </Button>
                <Button
                  disabled={!lookupResult.organization.id}
                  onClick={() => {
                    const organizationID = lookupResult.organization.id
                    if (!organizationID) return
                    setLookupOpen(false)
                    navigate(`/organizations/${organizationID}?tab=domains`)
                  }}
                >
                  进入组织并确认归属
                </Button>
              </DialogFooter>
            </div>
          ) : (
            <form
              onSubmit={(event) => {
                event.preventDefault()
                if (lookupDomain.trim()) lookup.mutate()
              }}
            >
              <Field label="已授权主域">
                <Input
                  autoFocus
                  value={lookupDomain}
                  onChange={(event) => setLookupDomain(event.target.value)}
                  placeholder="输入已授权主域，例如 guc.edu.cn"
                />
              </Field>
              <p className="mt-2 text-[11px] leading-5 text-muted-foreground">
                只查询你有权开展安全评估的域名；结果会保留来源、时间和原始证据供复核。
              </p>
              <DialogFooter>
                <Button type="button" variant="outline" onClick={() => setLookupOpen(false)}>
                  取消
                </Button>
                <Button type="submit" disabled={!lookupDomain.trim() || lookup.isPending}>
                  {lookup.isPending ? '查询中…' : '查询归属'}
                </Button>
              </DialogFooter>
            </form>
          )}
        </DialogContent>
      </Dialog>

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>新建组织</DialogTitle>
            <DialogDescription>创建后可继续添加候选主域并执行 ICP 归属反查。</DialogDescription>
          </DialogHeader>
          <div className="space-y-3">
            <Field label="组织名称">
              <Input
                value={form.canonicalName}
                onChange={(event) => setForm({ ...form, canonicalName: event.target.value })}
              />
            </Field>
            <Field label="别名（用 | 分隔）">
              <Input value={form.aliases} onChange={(event) => setForm({ ...form, aliases: event.target.value })} />
            </Field>
            <div className="grid gap-3 sm:grid-cols-2">
              <Field label="地区">
                <Input value={form.region} onChange={(event) => setForm({ ...form, region: event.target.value })} />
              </Field>
              <Field label="统一社会信用代码">
                <Input value={form.uscc} onChange={(event) => setForm({ ...form, uscc: event.target.value })} />
              </Field>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateOpen(false)}>
              取消
            </Button>
            <Button disabled={!form.canonicalName.trim() || create.isPending} onClick={() => create.mutate()}>
              {create.isPending ? '创建中…' : '创建组织'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={importOpen} onOpenChange={setImportOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>导入组织 CSV</DialogTitle>
            <DialogDescription>
              必填列 canonicalName；可选 aliases、region、uscc、rootDomains。多个别名用 | 分隔，多个主域用分号分隔。
            </DialogDescription>
          </DialogHeader>
          <div className="rounded-md border bg-muted/40 p-3 text-[11px] leading-5 text-muted-foreground">
            <div className="mono">canonicalName,aliases,region,uscc,rootDomains</div>
            <div className="mono">四川某学院,学院A|学院B,蜀,12510000...,guc.edu.cn;example.edu.cn</div>
          </div>
          <Input
            type="file"
            accept=".csv,text/csv"
            onChange={(event) => {
              const file = event.target.files?.[0]
              if (file) importOrgs.mutate(file)
            }}
          />
          <DialogFooter>
            <Button variant="outline" onClick={() => setImportOpen(false)}>
              取消
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={editOpen} onOpenChange={setEditOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>编辑组织</DialogTitle>
            <DialogDescription>名称、别名、地区和统一社会信用代码会同步到组织治理与图谱。</DialogDescription>
          </DialogHeader>
          <div className="space-y-3">
            <Field label="组织名称">
              <Input
                value={form.canonicalName}
                onChange={(event) => setForm({ ...form, canonicalName: event.target.value })}
              />
            </Field>
            <Field label="别名（用 | 分隔）">
              <Input value={form.aliases} onChange={(event) => setForm({ ...form, aliases: event.target.value })} />
            </Field>
            <div className="grid gap-3 sm:grid-cols-2">
              <Field label="地区">
                <Input value={form.region} onChange={(event) => setForm({ ...form, region: event.target.value })} />
              </Field>
              <Field label="统一社会信用代码">
                <Input value={form.uscc} onChange={(event) => setForm({ ...form, uscc: event.target.value })} />
              </Field>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setEditOpen(false)}>
              取消
            </Button>
            <Button disabled={!form.canonicalName.trim() || update.isPending} onClick={() => update.mutate()}>
              {update.isPending ? '保存中…' : '保存修改'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="space-y-1.5">
      <Label>{label}</Label>
      {children}
    </div>
  )
}

function confidenceLabel(value?: string) {
  switch (value) {
    case 'high':
      return '高置信度'
    case 'medium':
      return '中置信度'
    case 'low':
      return '低置信度'
    default:
      return '置信度未知'
  }
}

function providerLabel(value?: string) {
  if (value === 'chinaz') return '站长 ICP'
  if (value === 'icp_query') return 'ICP_Query'
  return '备案数据源'
}
