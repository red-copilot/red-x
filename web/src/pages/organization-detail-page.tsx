import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowLeft, Download, FileClock, Globe2, Play, PlusCircle, RefreshCw, ShieldAlert } from 'lucide-react'
import { useState } from 'react'
import { Link, useParams, useSearchParams } from 'react-router-dom'
import { toast } from 'sonner'
import { api, type EasmAsset, type FindingItem, type OrganizationDomain, type Task } from '@/api'
import { download } from '@/api/client'
import { RiskBadge, StatusBadge } from '@/components/common/badges'
import { DataTable } from '@/components/common/data-table'
import { EmptyState, ErrorState, LoadingBlock } from '@/components/common/states'
import { StatTile } from '@/components/common/stat-tile'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import { canWrite } from '@/lib/access'
import { useAuth } from '@/app/auth'
import { formatNumber, formatTime, relativeTime } from '@/lib/format'

const tabs = ['overview', 'domains', 'assets', 'findings', 'tasks'] as const

export function OrganizationDetailPage() {
  const { id = '' } = useParams()
  const { role } = useAuth()
  const queryClient = useQueryClient()
  const [params, setParams] = useSearchParams()
  const tab = tabs.includes((params.get('tab') as (typeof tabs)[number]) || 'overview')
    ? (params.get('tab') as (typeof tabs)[number])
    : 'overview'
  const [addOpen, setAddOpen] = useState(false)
  const [scanOpen, setScanOpen] = useState(false)
  const [evidenceView, setEvidenceView] = useState<unknown>(null)
  const [rejectTarget, setRejectTarget] = useState<OrganizationDomain | null>(null)
  const [rejectReason, setRejectReason] = useState('人工移出')
  const [domainText, setDomainText] = useState('')
  const [selectedDomains, setSelectedDomains] = useState<string[]>([])
  const [scanDomains, setScanDomains] = useState<string[]>([])

  const organization = useQuery({
    queryKey: ['organization', id],
    queryFn: () => api.organization(id),
    enabled: Boolean(id),
  })
  const summary = useQuery({
    queryKey: ['organization-summary', id],
    queryFn: () => api.organizationSummary(id),
    enabled: Boolean(id),
  })
  const domains = useQuery({
    queryKey: ['organization-domains', id],
    queryFn: () => api.domains(id),
    enabled: Boolean(id),
  })
  const assets = useQuery({
    queryKey: ['organization-assets', id],
    queryFn: () => api.organizationAssets(id, { page: 1, pageSize: 100 }),
    enabled: Boolean(id) && tab === 'assets',
  })
  const findings = useQuery({
    queryKey: ['organization-findings', id],
    queryFn: () => api.organizationFindings(id, { view: 'current', page: 1, pageSize: 100 }),
    enabled: Boolean(id) && tab === 'findings',
  })
  const tasks = useQuery({
    queryKey: ['organization-tasks', id],
    queryFn: () => api.organizationTasks(id, { page: 1, pageSize: 100 }),
    enabled: Boolean(id) && tab === 'tasks',
  })

  const refreshAll = () => {
    void queryClient.invalidateQueries({ queryKey: ['organization', id] })
    void queryClient.invalidateQueries({ queryKey: ['organization-domains', id] })
    void queryClient.invalidateQueries({ queryKey: ['organization-summary', id] })
    void queryClient.invalidateQueries({ queryKey: ['organization-assets', id] })
    void queryClient.invalidateQueries({ queryKey: ['organization-findings', id] })
    void queryClient.invalidateQueries({ queryKey: ['organization-tasks', id] })
  }

  const confirm = useMutation({
    mutationFn: () => api.confirmDomains(id, selectedDomains),
    onSuccess: () => {
      toast.success('域名归属已确认')
      setSelectedDomains([])
      refreshAll()
    },
    onError: (error) => toast.error(error.message),
  })
  const addDomains = useMutation({
    mutationFn: () => api.importDomains(id, domainText.split(/[\s,]+/).filter(Boolean)),
    onSuccess: () => {
      toast.success('候选域名已加入')
      setDomainText('')
      setAddOpen(false)
      refreshAll()
    },
    onError: (error) => toast.error(error.message),
  })
  const refreshICP = useMutation({
    mutationFn: () => api.refreshICP(id),
    onSuccess: () => {
      toast.success('ICP 反查任务已提交')
      refreshAll()
    },
    onError: (error) => toast.error(error.message),
  })
  const reject = useMutation({
    mutationFn: ({ domainId, reason }: { domainId: string; reason: string }) => api.rejectDomain(id, domainId, reason),
    onSuccess: () => {
      toast.success('域名已移出')
      refreshAll()
    },
    onError: (error) => toast.error(error.message),
  })
  const createScans = useMutation({
    mutationFn: () => api.createScans(id, scanDomains),
    onSuccess: () => {
      toast.success(`已创建 ${scanDomains.length} 个扫描任务`)
      setScanOpen(false)
      setScanDomains([])
      refreshAll()
    },
    onError: (error) => toast.error(error.message),
  })
  const exportReport = useMutation({
    mutationFn: (format: 'xlsx' | 'csv') => api.exportOrganization(id, format),
    onSuccess: (result) => void download(result.downloadUrl || ''),
    onError: (error) => toast.error(error.message),
  })

  if (organization.isLoading || summary.isLoading) return <LoadingBlock label="正在加载组织工作区…" />
  if (organization.error || summary.error)
    return (
      <ErrorState
        error={organization.error || summary.error}
        retry={() => {
          organization.refetch()
          summary.refetch()
        }}
      />
    )
  const org = organization.data!.organization
  const domainItems = domains.data?.items || []
  const confirmed = domainItems
    .filter((domain) => domain.status === 'confirmed')
    .map((domain) => domain.rootDomain)
    .filter((domain): domain is string => Boolean(domain))

  function switchTab(next: string) {
    const search = new URLSearchParams(params)
    if (next === 'overview') search.delete('tab')
    else search.set('tab', next)
    setParams(search, { replace: true })
  }
  async function showEvidence(domain: OrganizationDomain) {
    try {
      setEvidenceView(await api.evidence(id, domain.id || ''))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '证据加载失败')
    }
  }

  return (
    <div>
      <div className="mb-4 flex items-center gap-2 text-xs text-muted-foreground">
        <Button asChild variant="ghost" size="sm">
          <Link to="/organizations">
            <ArrowLeft />
            组织列表
          </Link>
        </Button>
        <span>/</span>
        <span>{org.canonicalName}</span>
      </div>
      <div className="mb-5 flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
        <div>
          <div className="flex flex-wrap items-center gap-2">
            <h1 className="text-2xl font-semibold tracking-tight">{org.canonicalName}</h1>
            <StatusBadge status={org.status} />
          </div>
          <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
            {org.region ? <span>地区：{org.region}</span> : null}
            {org.aliases?.length ? <span>别名：{org.aliases.join(' / ')}</span> : null}
            {org.uscc ? <span className="mono">USCC：{org.uscc}</span> : null}
          </div>
        </div>
        {canWrite(role) ? (
          <div className="flex flex-wrap gap-2">
            <Button variant="outline" onClick={() => refreshICP.mutate()} disabled={refreshICP.isPending}>
              <RefreshCw />
              反查备案域
            </Button>
            <Button variant="outline" onClick={() => setAddOpen(true)}>
              <PlusCircle />
              添加主域
            </Button>
            <Button variant="outline" onClick={() => exportReport.mutate('xlsx')}>
              <Download />
              导出
            </Button>
            <Button
              onClick={() => {
                setScanDomains(confirmed.slice(0, 10))
                setScanOpen(true)
              }}
              disabled={!confirmed.length}
            >
              <Play />
              发起扫描
            </Button>
          </div>
        ) : null}
      </div>

      <div className="mb-4 grid grid-cols-2 gap-3 lg:grid-cols-6">
        <StatTile
          label="已确认主域"
          value={summary.data?.confirmedDomains || 0}
          hint={`待确认 ${summary.data?.candidateDomains || 0}`}
          icon={<Globe2 className="size-4" />}
        />
        <StatTile label="域名资产" value={formatNumber(summary.data?.domainAssets)} />
        <StatTile
          label="IP 资产"
          value={formatNumber(summary.data?.ipAssets)}
          hint={`未定位 ${summary.data?.unlocatedAssets || 0}`}
        />
        <StatTile label="服务" value={formatNumber(summary.data?.services)} />
        <StatTile
          label="当前风险"
          value={formatNumber(summary.data?.currentFindings)}
          hint={`严重 ${summary.data?.criticalFindings || 0} · 高危 ${summary.data?.highFindings || 0}`}
          icon={<ShieldAlert className="size-4" />}
        />
        <StatTile
          label="最近扫描"
          value={relativeTime(summary.data?.lastScanAt)}
          hint={`${summary.data?.linkedTasks || 0} 个关联任务`}
          icon={<FileClock className="size-4" />}
        />
      </div>

      <Tabs value={tab} onValueChange={switchTab}>
        <TabsList>
          <TabsTrigger value="overview">概览</TabsTrigger>
          <TabsTrigger value="domains">主域与归属</TabsTrigger>
          <TabsTrigger value="assets">资产</TabsTrigger>
          <TabsTrigger value="findings">风险</TabsTrigger>
          <TabsTrigger value="tasks">任务</TabsTrigger>
        </TabsList>
        <TabsContent value="overview">
          <div className="grid gap-4 lg:grid-cols-2">
            <Card>
              <CardHeader>
                <CardTitle>归属覆盖</CardTitle>
              </CardHeader>
              <CardContent className="space-y-3 text-xs">
                <Coverage
                  label="已确认主域"
                  value={summary.data?.confirmedDomains || 0}
                  total={(summary.data?.confirmedDomains || 0) + (summary.data?.candidateDomains || 0)}
                />
                <Coverage
                  label="已有资产发现"
                  value={(summary.data?.domainAssets || 0) + (summary.data?.ipAssets || 0) > 0 ? 1 : 0}
                  total={1}
                />
                <Coverage label="已开展扫描" value={(summary.data?.linkedTasks || 0) > 0 ? 1 : 0} total={1} />
              </CardContent>
            </Card>
            <Card>
              <CardHeader>
                <CardTitle>风险构成</CardTitle>
              </CardHeader>
              <CardContent className="space-y-2 text-xs">
                {(
                  [
                    ['critical', '严重'],
                    ['high', '高危'],
                    ['medium', '中危'],
                    ['low', '低危'],
                    ['info', '信息'],
                  ] as const
                ).map(([key, label]) => (
                  <div key={key} className="flex items-center justify-between border-b pb-2 last:border-0">
                    <span className={`risk-${key}`}>{label}</span>
                    <span className="tabular-nums">
                      {(summary.data?.[`${key}Findings` as keyof typeof summary.data] as number) || 0}
                    </span>
                  </div>
                ))}
              </CardContent>
            </Card>
          </div>
        </TabsContent>
        <TabsContent value="domains">
          <Card>
            <CardHeader className="flex-row items-center justify-between space-y-0">
              <CardTitle>备案主域</CardTitle>
              <div className="flex gap-2">
                <Button size="sm" variant="outline" onClick={() => setAddOpen(true)}>
                  <PlusCircle />
                  添加
                </Button>
                <Button size="sm" disabled={!selectedDomains.length} onClick={() => confirm.mutate()}>
                  确认所选
                </Button>
              </div>
            </CardHeader>
            <CardContent>
              {domains.isLoading ? (
                <LoadingBlock />
              ) : (
                <DataTable
                  rows={domainItems}
                  rowKey={(row) => row.id || row.rootDomain || ''}
                  empty={<EmptyState title="暂无备案主域" description="添加候选域名或执行 ICP 反查。" />}
                  columns={[
                    {
                      key: 'select',
                      header: '',
                      cell: (row) =>
                        row.status === 'candidate' ? (
                          <Checkbox
                            checked={selectedDomains.includes(row.rootDomain || '')}
                            onCheckedChange={(checked) =>
                              setSelectedDomains((current) =>
                                checked
                                  ? [...current, row.rootDomain || '']
                                  : current.filter((item) => item !== row.rootDomain),
                              )
                            }
                          />
                        ) : null,
                    },
                    {
                      key: 'domain',
                      header: '主域',
                      cell: (row) => <span className="mono font-medium">{row.rootDomain}</span>,
                    },
                    { key: 'site', header: '站点', cell: (row) => row.siteName || '—', hideOnTablet: true },
                    { key: 'icp', header: '备案号', cell: (row) => row.icpNo || '—', hideOnTablet: true },
                    { key: 'confidence', header: '置信度', cell: (row) => row.confidence || '—' },
                    { key: 'status', header: '状态', cell: (row) => <StatusBadge status={row.status} /> },
                    {
                      key: 'actions',
                      header: '操作',
                      cell: (row) => (
                        <div className="flex flex-wrap justify-end gap-1">
                          <Button size="sm" variant="ghost" onClick={() => void showEvidence(row)}>
                            证据
                          </Button>
                          {canWrite(role) && row.status === 'candidate' ? (
                            <Button
                              size="sm"
                              variant="ghost"
                              className="text-destructive"
                              onClick={() => {
                                setRejectReason('人工移出')
                                setRejectTarget(row)
                              }}
                            >
                              移出
                            </Button>
                          ) : null}
                        </div>
                      ),
                    },
                  ]}
                />
              )}
            </CardContent>
          </Card>
        </TabsContent>
        <TabsContent value="assets">
          <AssetTab query={assets} />
        </TabsContent>
        <TabsContent value="findings">
          <FindingTab query={findings} organizationId={id} />
        </TabsContent>
        <TabsContent value="tasks">
          <TaskTab query={tasks} />
        </TabsContent>
      </Tabs>

      <Dialog open={addOpen} onOpenChange={setAddOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>添加候选主域</DialogTitle>
            <DialogDescription>支持空格、换行或逗号分隔。新域名会进入待确认状态。</DialogDescription>
          </DialogHeader>
          <Textarea
            value={domainText}
            onChange={(event) => setDomainText(event.target.value)}
            placeholder="example.edu.cn"
          />
          <DialogFooter>
            <Button variant="outline" onClick={() => setAddOpen(false)}>
              取消
            </Button>
            <Button disabled={!domainText.trim() || addDomains.isPending} onClick={() => addDomains.mutate()}>
              {addDomains.isPending ? '添加中…' : '添加主域'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      <Dialog open={scanOpen} onOpenChange={setScanOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>发起组织扫描</DialogTitle>
            <DialogDescription>每个已确认主域创建一个 EASM 扫描任务，最多同时选择 10 个。</DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            {confirmed.map((domain) => (
              <label key={domain} className="flex items-center gap-2 rounded-md border px-3 py-2 text-xs">
                <Checkbox
                  checked={scanDomains.includes(domain)}
                  onCheckedChange={(checked) =>
                    setScanDomains((current) =>
                      checked ? current.slice(0, 9).concat(domain) : current.filter((item) => item !== domain),
                    )
                  }
                />
                <span className="mono">{domain}</span>
              </label>
            ))}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setScanOpen(false)}>
              取消
            </Button>
            <Button disabled={!scanDomains.length || createScans.isPending} onClick={() => createScans.mutate()}>
              {createScans.isPending ? '提交中…' : '创建扫描任务'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      <Dialog open={Boolean(rejectTarget)} onOpenChange={(open) => !open && setRejectTarget(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>移出候选主域</DialogTitle>
            <DialogDescription>
              {rejectTarget?.rootDomain} 将从当前组织的候选归属中移除，并保留操作原因。
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-1.5">
            <Label>原因</Label>
            <Input value={rejectReason} onChange={(event) => setRejectReason(event.target.value)} />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRejectTarget(null)}>
              取消
            </Button>
            <Button
              variant="destructive"
              disabled={!rejectReason.trim() || reject.isPending}
              onClick={() => {
                reject.mutate({ domainId: rejectTarget?.id || '', reason: rejectReason.trim() })
                setRejectTarget(null)
              }}
            >
              移出主域
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      <Dialog open={evidenceView !== null} onOpenChange={(open) => !open && setEvidenceView(null)}>
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>归属证据</DialogTitle>
            <DialogDescription>原始证据仅对操作员和管理员展示敏感字段。</DialogDescription>
          </DialogHeader>
          <pre className="max-h-[55vh] overflow-auto rounded-md bg-slate-950 p-3 text-[11px] leading-5 text-slate-100">
            {JSON.stringify(evidenceView, null, 2)}
          </pre>
        </DialogContent>
      </Dialog>
    </div>
  )
}

function Coverage({ label, value, total }: { label: string; value: number; total: number }) {
  const pct = total ? Math.round((value / total) * 100) : 0
  return (
    <div>
      <div className="mb-1.5 flex justify-between">
        <span className="text-muted-foreground">{label}</span>
        <b>
          {value} / {total}
        </b>
      </div>
      <div className="h-1.5 overflow-hidden rounded-full bg-muted">
        <div className="h-full bg-primary" style={{ width: `${pct}%` }} />
      </div>
    </div>
  )
}
function AssetTab({ query }: { query: ReturnType<typeof useQuery<any>> }) {
  if (query.isLoading) return <LoadingBlock />
  const items = query.data?.items || []
  return (
    <Card>
      <CardContent className="pt-4">
        <DataTable
          rows={items}
          rowKey={(row: EasmAsset) => row.id || row.key || ''}
          empty={<EmptyState title="暂无资产" />}
          columns={[
            { key: 'key', header: '资产', cell: (row: EasmAsset) => <span className="mono">{row.key}</span> },
            { key: 'kind', header: '类型', cell: (row: EasmAsset) => row.kind },
            { key: 'risk', header: '风险', cell: (row: EasmAsset) => <RiskBadge risk={row.risk} /> },
            { key: 'seen', header: '最近发现', cell: (row: EasmAsset) => formatTime(row.lastSeen) },
          ]}
        />
      </CardContent>
    </Card>
  )
}
function FindingTab({ query, organizationId }: { query: ReturnType<typeof useQuery<any>>; organizationId: string }) {
  if (query.isLoading) return <LoadingBlock />
  const items = query.data?.items || []
  async function history(item: FindingItem) {
    try {
      const result = await api.findingHistory(item.finding?.id || '', organizationId)
      toast.message(`历史记录 ${result.items.length} 条`, { description: result.items[0]?.title })
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '历史加载失败')
    }
  }
  return (
    <Card>
      <CardContent className="pt-4">
        <DataTable
          rows={items}
          rowKey={(row: FindingItem) => row.finding?.id || ''}
          empty={<EmptyState title="暂无当前风险" />}
          columns={[
            {
              key: 'title',
              header: '风险',
              cell: (row: FindingItem) => (
                <div>
                  <div className="font-medium">{row.finding?.title}</div>
                  <div className="text-[11px] text-muted-foreground">{row.finding?.target}</div>
                </div>
              ),
            },
            { key: 'severity', header: '级别', cell: (row: FindingItem) => <RiskBadge risk={row.finding?.severity} /> },
            { key: 'occurrences', header: '次数', cell: (row: FindingItem) => row.occurrences || 1 },
            { key: 'time', header: '最近发现', cell: (row: FindingItem) => formatTime(row.finding?.createdAt) },
            {
              key: 'actions',
              header: '操作',
              cell: (row: FindingItem) => (
                <Button size="sm" variant="ghost" onClick={() => void history(row)}>
                  历史
                </Button>
              ),
            },
          ]}
        />
      </CardContent>
    </Card>
  )
}
function TaskTab({ query }: { query: ReturnType<typeof useQuery<any>> }) {
  if (query.isLoading) return <LoadingBlock />
  const items = query.data?.items || []
  return (
    <Card>
      <CardContent className="pt-4">
        <DataTable
          rows={items}
          rowKey={(row: Task) => row.id || ''}
          empty={<EmptyState title="暂无任务" />}
          columns={[
            {
              key: 'name',
              header: '任务',
              cell: (row: Task) => (
                <Link className="font-medium text-primary hover:underline" to={`/tasks/${row.id}`}>
                  {row.name}
                </Link>
              ),
            },
            { key: 'target', header: '目标', cell: (row: Task) => <span className="mono">{row.target}</span> },
            { key: 'status', header: '状态', cell: (row: Task) => <StatusBadge status={row.status} /> },
            { key: 'progress', header: '进度', cell: (row: Task) => `${row.progress || 0}%` },
            { key: 'created', header: '创建时间', cell: (row: Task) => formatTime(row.createdAt) },
          ]}
        />
      </CardContent>
    </Card>
  )
}
