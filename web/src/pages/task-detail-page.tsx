import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowLeft, Download, RefreshCw, RotateCcw, Square } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Link, useParams, useSearchParams } from 'react-router-dom'
import { toast } from 'sonner'
import { api } from '@/api'
import { download } from '@/api/client'
import { RiskBadge, StatusBadge } from '@/components/common/badges'
import { DataTable } from '@/components/common/data-table'
import { EmptyState, ErrorState, LoadingBlock } from '@/components/common/states'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Progress } from '@/components/ui/progress'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { useAuth } from '@/app/auth'
import { canWrite } from '@/lib/access'
import { formatTime, relativeTime } from '@/lib/format'

type TaskEvent = { type?: string; message?: string; createdAt?: string; payload?: Record<string, unknown> }

export function TaskDetailPage() {
  const { id = '' } = useParams()
  const { role } = useAuth()
  const queryClient = useQueryClient()
  const [params, setParams] = useSearchParams()
  const [events, setEvents] = useState<TaskEvent[]>([])
  const tab = params.get('tab') || 'events'
  const detail = useQuery({
    queryKey: ['task', id],
    queryFn: () => api.task(id),
    enabled: Boolean(id),
    refetchInterval: 15_000,
  })
  const observations = useQuery({
    queryKey: ['task-observations', id, tab],
    queryFn: () => api.taskObservations(id, tab === 'ips' ? 'ip' : tab === 'assets' ? 'domain' : 'site'),
    enabled: ['domains', 'ips', 'sites', 'assets'].includes(tab),
  })
  const findings = useQuery({
    queryKey: ['task-findings', id, tab],
    queryFn: () => api.taskFindings(id, tab === 'wih' ? 'wih' : 'vuln'),
    enabled: tab === 'vulns' || tab === 'wih',
  })

  useEffect(() => {
    if (!id) return
    let source: EventSource | null = null
    let timer: ReturnType<typeof setTimeout> | undefined
    let retry = 1500
    let closed = false
    const connect = () => {
      source = new EventSource(`/api/v1/tasks/${id}/events`)
      const handle = (event: MessageEvent) => {
        try {
          const parsed = JSON.parse(event.data) as TaskEvent
          setEvents((current) => [...current.slice(-299), parsed])
          if (['task.status', 'step.status', 'task.warning'].includes(parsed.type || '')) {
            void queryClient.invalidateQueries({ queryKey: ['task', id] })
          }
        } catch {
          // Ignore malformed SSE frames and keep the stream alive.
        }
      }
      source.onmessage = handle
      for (const name of ['task.status', 'step.status', 'task.warning', 'asset.new', 'finding.new'])
        source.addEventListener(name, handle as EventListener)
      source.onopen = () => {
        retry = 1500
      }
      source.onerror = () => {
        source?.close()
        if (closed) return
        timer = setTimeout(connect, retry)
        retry = Math.min(retry * 2, 15_000)
      }
    }
    connect()
    return () => {
      closed = true
      source?.close()
      if (timer) clearTimeout(timer)
    }
  }, [id, queryClient])

  const exportTask = useMutation({
    mutationFn: () => api.createExport({ taskId: id, format: 'xlsx' }),
    onSuccess: (result) => void download(result.downloadUrl || ''),
    onError: (error) => toast.error(error.message),
  })
  const action = useMutation({
    mutationFn: async ({ kind }: { kind: 'cancel' | 'retry' | 'sync' }) => {
      if (kind === 'cancel') await api.cancelTask(id)
      else if (kind === 'retry') await api.retryTask(id)
      else await api.syncTask(id)
    },
    onSuccess: (_, variables) => {
      toast.success(
        variables.kind === 'cancel' ? '任务已停止' : variables.kind === 'retry' ? '已重新提交' : '已触发同步',
      )
      void queryClient.invalidateQueries({ queryKey: ['task', id] })
    },
    onError: (error) => toast.error(error.message),
  })

  if (detail.isLoading) return <LoadingBlock label="正在加载任务…" />
  if (detail.error) return <ErrorState error={detail.error} retry={() => detail.refetch()} />
  const task = detail.data!.task
  const steps = detail.data!.steps || []

  function switchTab(value: string) {
    const next = new URLSearchParams(params)
    if (value === 'events') next.delete('tab')
    else next.set('tab', value)
    setParams(next, { replace: true })
  }

  return (
    <div>
      <div className="mb-4 flex items-center gap-2 text-xs text-muted-foreground">
        <Button asChild size="sm" variant="ghost">
          <Link to="/tasks">
            <ArrowLeft />
            任务列表
          </Link>
        </Button>
        <span>/</span>
        <span className="truncate">{task.name || task.id}</span>
      </div>
      <div className="mb-5 flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
        <div>
          <div className="flex flex-wrap items-center gap-2">
            <h1 className="text-2xl font-semibold tracking-tight">{task.name || '扫描任务'}</h1>
            <StatusBadge status={task.status} />
          </div>
          <div className="mono mt-2 text-xs text-muted-foreground">
            {task.target} · {task.kind} · {task.origin || 'manual'}
          </div>
        </div>
        {canWrite(role) ? (
          <div className="flex flex-wrap gap-2">
            <Button variant="outline" onClick={() => exportTask.mutate()}>
              <Download />
              导出
            </Button>
            <Button variant="outline" onClick={() => action.mutate({ kind: 'sync' })}>
              <RefreshCw />
              同步
            </Button>
            <Button
              variant="outline"
              disabled={['succeeded', 'failed', 'canceled', 'partial'].includes(task.status || '')}
              onClick={() => action.mutate({ kind: 'cancel' })}
            >
              <Square />
              停止
            </Button>
            {['failed', 'partial'].includes(task.status || '') ? (
              <Button onClick={() => action.mutate({ kind: 'retry' })}>
                <RotateCcw />
                重试
              </Button>
            ) : null}
          </div>
        ) : null}
      </div>
      <Card className="mb-4">
        <CardContent className="pt-4">
          <div className="mb-2 flex items-center justify-between text-xs">
            <span className="text-muted-foreground">任务进度</span>
            <b className="tabular-nums">{task.progress || 0}%</b>
          </div>
          <Progress value={task.progress || 0} />
          <div className="mt-4 grid grid-cols-2 gap-3 text-xs sm:grid-cols-4">
            <Fact label="创建时间" value={formatTime(task.createdAt)} />
            <Fact label="开始时间" value={formatTime(task.startedAt)} />
            <Fact label="完成时间" value={formatTime(task.finishedAt)} />
            <Fact label="策略" value={task.policyId || '默认'} />
          </div>
          {task.error ? (
            <div className="mt-3 rounded-md border border-[#f1b8b2] bg-[#fff8f7] px-3 py-2 text-xs text-[#8f1c13]">
              {task.error}
            </div>
          ) : null}
        </CardContent>
      </Card>
      <Tabs value={tab} onValueChange={switchTab}>
        <TabsList className="flex flex-wrap">
          <TabsTrigger value="events">实时事件</TabsTrigger>
          <TabsTrigger value="steps">步骤</TabsTrigger>
          <TabsTrigger value="domains">域名</TabsTrigger>
          <TabsTrigger value="ips">IP</TabsTrigger>
          <TabsTrigger value="sites">站点</TabsTrigger>
          <TabsTrigger value="vulns">漏洞</TabsTrigger>
          <TabsTrigger value="wih">WIH</TabsTrigger>
        </TabsList>
        <TabsContent value="events">
          <Card>
            <CardHeader>
              <CardTitle>事件流</CardTitle>
            </CardHeader>
            <CardContent>
              {events.length ? (
                <div className="max-h-[520px] space-y-2 overflow-auto pr-2 scrollbar-thin">
                  {events
                    .slice()
                    .reverse()
                    .map((event, index) => (
                      <div key={`${event.createdAt}-${index}`} className="rounded-md border p-2.5 text-xs">
                        <div className="flex justify-between gap-2">
                          <span className="font-medium">{event.type || 'event'}</span>
                          <span className="text-[10px] text-muted-foreground">{relativeTime(event.createdAt)}</span>
                        </div>
                        <div className="mt-1 text-muted-foreground">
                          {String(event.message || JSON.stringify(event.payload || {}))}
                        </div>
                      </div>
                    ))}
                </div>
              ) : (
                <EmptyState title="尚无实时事件" description="任务开始后事件会通过 SSE 实时推送。" />
              )}
            </CardContent>
          </Card>
        </TabsContent>
        <TabsContent value="steps">
          <Card>
            <CardContent className="pt-4">
              <div className="space-y-2">
                {steps.map((step) => (
                  <div key={step.id} className="rounded-md border p-3">
                    <div className="flex items-center justify-between gap-3">
                      <div>
                        <div className="text-xs font-medium">
                          {String(step.sequence)}. {String(step.kind)}
                        </div>
                        <div className="mt-1 text-[11px] text-muted-foreground">{String(step.message || '—')}</div>
                      </div>
                      <StatusBadge status={step.status} />
                    </div>
                    <Progress className="mt-2" value={step.progress || 0} />
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>
        </TabsContent>
        <TabsContent value="domains">
          <ObservationTab query={observations} />
        </TabsContent>
        <TabsContent value="ips">
          <ObservationTab query={observations} />
        </TabsContent>
        <TabsContent value="sites">
          <ObservationTab query={observations} />
        </TabsContent>
        <TabsContent value="vulns">
          <FindingTab query={findings} />
        </TabsContent>
        <TabsContent value="wih">
          <FindingTab query={findings} />
        </TabsContent>
      </Tabs>
    </div>
  )
}

function Fact({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <div className="text-[10px] text-muted-foreground">{label}</div>
      <div className="mt-1 font-medium">{value}</div>
    </div>
  )
}
function ObservationTab({ query }: { query: ReturnType<typeof useQuery<any>> }) {
  if (query.isLoading) return <LoadingBlock />
  const rows = query.data?.items || []
  return (
    <Card>
      <CardContent className="pt-4">
        <DataTable
          rows={rows}
          rowKey={(row: any) => row.id || row.key}
          empty={<EmptyState title="暂无观测结果" />}
          columns={[
            { key: 'key', header: '资产', cell: (row: any) => <span className="mono">{row.key}</span> },
            { key: 'source', header: '来源', cell: (row: any) => row.source || '—' },
            { key: 'time', header: '观测时间', cell: (row: any) => formatTime(row.observedAt) },
          ]}
        />
      </CardContent>
    </Card>
  )
}
function FindingTab({ query }: { query: ReturnType<typeof useQuery<any>> }) {
  if (query.isLoading) return <LoadingBlock />
  const rows = query.data?.items || []
  return (
    <Card>
      <CardContent className="pt-4">
        <DataTable
          rows={rows}
          rowKey={(row: any) => row.id || row.dedupeKey}
          empty={<EmptyState title="暂无发现" />}
          columns={[
            { key: 'title', header: '标题', cell: (row: any) => row.title },
            { key: 'target', header: '目标', cell: (row: any) => <span className="mono">{row.target}</span> },
            { key: 'severity', header: '级别', cell: (row: any) => <RiskBadge risk={row.severity} /> },
            { key: 'time', header: '时间', cell: (row: any) => formatTime(row.createdAt) },
          ]}
        />
      </CardContent>
    </Card>
  )
}
