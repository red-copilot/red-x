import { useQuery } from '@tanstack/react-query'
import {
  Activity,
  AlertTriangle,
  ArrowUpRight,
  Building2,
  Crosshair,
  Map as MapIcon,
  Network,
  RefreshCw,
  ShieldAlert,
} from 'lucide-react'
import { useMemo, useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { api } from '@/api'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { RiskBadge } from '@/components/common/badges'
import { ErrorState, LoadingBlock } from '@/components/common/states'
import { StatTile } from '@/components/common/stat-tile'
import { AssetGraph } from '@/components/visuals/asset-graph'
import { SurfaceMap } from '@/components/visuals/surface-map'
import { formatNumber, formatTime, relativeTime, riskLabel } from '@/lib/format'
import { cn } from '@/lib/utils'

const ranges = [
  { value: '24h', label: '近 24 小时' },
  { value: '7d', label: '近 7 天' },
  { value: '30d', label: '近 30 天' },
  { value: '90d', label: '近 90 天' },
  { value: 'all', label: '全部' },
]

export function OverviewPage() {
  const [params, setParams] = useSearchParams()
  const navigate = useNavigate()
  const [range, setRange] = useState('30d')
  const [visual, setVisual] = useState<'map' | 'graph'>('map')
  const selectedOrg = params.get('organizationId') || ''
  const overview = useQuery({
    queryKey: ['overview', range],
    queryFn: () => api.overview(range),
    refetchInterval: 30_000,
  })
  const map = useQuery({
    queryKey: ['map', selectedOrg],
    queryFn: () => api.map(selectedOrg || undefined),
    refetchInterval: 45_000,
  })
  const graph = useQuery({
    queryKey: ['graph', selectedOrg],
    queryFn: () => api.graph(selectedOrg),
    enabled: Boolean(selectedOrg) && visual === 'graph',
    retry: 0,
  })
  const summary = useQuery({
    queryKey: ['organization-summary', selectedOrg],
    queryFn: () => api.organizationSummary(selectedOrg),
    enabled: Boolean(selectedOrg),
  })
  const selectedRisk = useMemo(
    () => overview.data?.topOrganizations.find((item) => item.organizationId === selectedOrg),
    [overview.data, selectedOrg],
  )

  if (overview.isLoading) return <LoadingBlock label="正在汇总攻击面…" />
  if (overview.error) return <ErrorState error={overview.error} retry={() => overview.refetch()} />

  const data = overview.data!
  const critical = (data.severity.critical || 0) + (data.severity.high || 0)
  const health = data.health

  function selectOrg(orgId: string) {
    const next = new URLSearchParams(params)
    if (orgId) next.set('organizationId', orgId)
    else next.delete('organizationId')
    setParams(next, { replace: true })
  }

  return (
    <div>
      <div className="mb-5 flex flex-col gap-3 xl:flex-row xl:items-end xl:justify-between">
        <div>
          <div className="mb-1 flex items-center gap-2 text-[11px] font-medium uppercase tracking-[0.12em] text-primary">
            <Activity className="size-3.5" />
            Attack Surface Overview
          </div>
          <h1 className="text-xl font-semibold tracking-tight sm:text-2xl">外部攻击面总览</h1>
          <p className="mt-1 text-sm text-muted-foreground">先看单位和当前风险，再沿地图、关系图和资产列表进入研判。</p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Select value={range} onValueChange={setRange}>
            <SelectTrigger className="w-36">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {ranges.map((item) => (
                <SelectItem key={item.value} value={item.value}>
                  {item.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Button
            variant="outline"
            onClick={() => {
              overview.refetch()
              map.refetch()
            }}
          >
            <RefreshCw />
            刷新
          </Button>
          <Button asChild>
            <Link to="/organizations">
              <Building2 />
              组织工作区
            </Link>
          </Button>
        </div>
      </div>

      <div className="mb-4 grid grid-cols-2 gap-3 lg:grid-cols-5">
        <StatTile
          label="组织"
          value={formatNumber(data.summary.organizations)}
          hint={`${formatNumber(data.summary.confirmedDomains)} 个已确认主域`}
          icon={<Building2 className="size-4" />}
        />
        <StatTile
          label="暴露面资产"
          value={formatNumber(
            (data.summary.domainAssets || 0) + (data.summary.ipAssets || 0) + (data.summary.services || 0),
          )}
          hint={`${formatNumber(data.summary.unlocatedAssets)} 个未定位 IP`}
          icon={<Crosshair className="size-4" />}
        />
        <StatTile
          label="当前风险"
          value={formatNumber(data.summary.currentFindings)}
          hint={`严重/高危 ${formatNumber(critical)}`}
          icon={<ShieldAlert className="size-4" />}
        />
        <StatTile
          label="运行任务"
          value={formatNumber((data.tasks.running || 0) + (data.tasks.queued || 0))}
          hint={`已完成 ${formatNumber(data.tasks.succeeded || 0)}`}
          icon={<Activity className="size-4" />}
        />
        <StatTile
          label="数据新鲜度"
          value={relativeTime(data.updatedAt)}
          hint={formatTime(data.updatedAt)}
          icon={<RefreshCw className="size-4" />}
          testId="overview-freshness"
        />
      </div>

      <div className="grid min-h-[620px] gap-4 xl:grid-cols-[260px_minmax(0,1fr)_320px]">
        <Card className="order-2 overflow-hidden xl:order-1">
          <CardHeader className="flex-row items-center justify-between space-y-0 border-b py-3">
            <CardTitle>风险组织</CardTitle>
            <Badge variant="outline">{data.topOrganizations.length}</Badge>
          </CardHeader>
          <div className="max-h-[560px] overflow-auto p-2 scrollbar-thin">
            {data.topOrganizations.length ? (
              data.topOrganizations.map((org) => (
                <button
                  key={org.organizationId}
                  className={cn(
                    'mb-1 w-full rounded-md border border-transparent px-3 py-2.5 text-left transition-colors hover:bg-muted',
                    selectedOrg === org.organizationId && 'border-[#bfd4ee] bg-[#f1f6fc]',
                  )}
                  onClick={() => selectOrg(org.organizationId)}
                >
                  <div className="flex items-start justify-between gap-2">
                    <div className="min-w-0">
                      <div className="truncate text-xs font-medium">{org.name}</div>
                      <div className="mt-1 text-[11px] text-muted-foreground">
                        {org.assetCount || 0} 资产 · {org.findingCount || 0} 风险
                      </div>
                    </div>
                    <RiskBadge risk={org.risk} />
                  </div>
                </button>
              ))
            ) : (
              <div className="px-3 py-12 text-center text-xs text-muted-foreground">
                暂无组织数据
                <br />
                <Link className="mt-2 inline-block text-primary hover:underline" to="/organizations">
                  导入 CSV
                </Link>
              </div>
            )}
          </div>
        </Card>

        <Card className="order-1 flex min-h-[620px] flex-col overflow-hidden xl:order-2">
          <div className="flex flex-wrap items-center justify-between gap-2 border-b px-3 py-2.5">
            <div className="flex items-center gap-1 rounded-md bg-muted p-1">
              <Button size="sm" variant={visual === 'map' ? 'default' : 'ghost'} onClick={() => setVisual('map')}>
                <MapIcon />
                地图
              </Button>
              <Button
                size="sm"
                variant={visual === 'graph' ? 'default' : 'ghost'}
                disabled={!selectedOrg}
                onClick={() => setVisual('graph')}
              >
                <Network />
                关系图
              </Button>
            </div>
            <div className="flex flex-wrap items-center gap-1.5">
              <HealthChip label="ICP" value={health.icpQuery} />
              <HealthChip label="GeoIP" value={health.geoip} />
              <HealthChip label="Neo4j" value={health.neo4j} />
              {selectedOrg ? (
                <Button size="sm" variant="ghost" onClick={() => selectOrg('')}>
                  全局
                </Button>
              ) : null}
            </div>
          </div>
          <div className="min-h-0 flex-1 p-3">
            {visual === 'map' ? (
              map.isLoading ? (
                <LoadingBlock label="正在定位资产…" />
              ) : (
                <SurfaceMap snapshot={map.data} onSelect={(_id, orgIds) => orgIds[0] && selectOrg(orgIds[0])} />
              )
            ) : graph.error ? (
              <div className="grid h-full min-h-[360px] place-items-center rounded-lg border bg-card p-6 text-center">
                <div>
                  <Network className="mx-auto mb-2 size-6 text-muted-foreground" />
                  <div className="text-sm font-medium">关系图暂不可用</div>
                  <p className="mt-1 text-xs text-muted-foreground">可继续通过组织、资产和风险列表完成研判。</p>
                </div>
              </div>
            ) : graph.isLoading ? (
              <LoadingBlock label="正在加载关系图…" />
            ) : (
              <AssetGraph
                snapshot={graph.data}
                onSelect={(node) =>
                  node.entityId &&
                  navigate(
                    node.type === 'organization' ? `/organizations/${node.entityId}` : `/assets?focus=${node.entityId}`,
                  )
                }
              />
            )}
          </div>
          <div className="flex flex-wrap items-center justify-between gap-2 border-t px-3 py-2 text-[10px] text-muted-foreground">
            <span>
              {map.data
                ? `未定位资产 ${formatNumber(map.data.unlocatedAssets)} · 未定位组织 ${formatNumber(map.data.unlocatedOrganizations)}`
                : '地理数据尚未加载'}
            </span>
            <span>{map.data?.truncated ? '数据已截断' : '地图与组织筛选共享上下文'}</span>
          </div>
        </Card>

        <div className="order-3 space-y-4">
          <Card>
            <CardHeader className="border-b py-3">
              <CardTitle>{selectedOrg ? '选中组织' : '最近发现'}</CardTitle>
            </CardHeader>
            <CardContent className="p-3">
              {selectedOrg ? (
                <div>
                  <div className="flex items-start justify-between gap-2">
                    <div className="text-sm font-semibold">{selectedRisk?.name || '组织详情'}</div>
                    <RiskBadge risk={selectedRisk?.risk} />
                  </div>
                  {summary.data ? (
                    <div className="mt-3 grid grid-cols-2 gap-2 text-xs">
                      <MiniFact label="主域" value={summary.data.confirmedDomains || 0} />
                      <MiniFact label="资产" value={(summary.data.domainAssets || 0) + (summary.data.ipAssets || 0)} />
                      <MiniFact label="当前风险" value={summary.data.currentFindings || 0} />
                      <MiniFact label="任务" value={summary.data.linkedTasks || 0} />
                    </div>
                  ) : null}
                  <Button asChild className="mt-4 w-full" variant="outline">
                    <Link to={`/organizations/${selectedOrg}`}>
                      打开组织工作区
                      <ArrowUpRight />
                    </Link>
                  </Button>
                </div>
              ) : data.recentFindings.length ? (
                <div className="space-y-3">
                  {data.recentFindings.slice(0, 5).map((item) => (
                    <div key={item.finding?.id} className="border-b pb-3 last:border-0 last:pb-0">
                      <div className="flex items-start justify-between gap-2">
                        <div className="line-clamp-2 text-xs font-medium">{item.finding?.title}</div>
                        <RiskBadge risk={item.finding?.severity} />
                      </div>
                      <div className="mt-1 truncate text-[11px] text-muted-foreground">{item.finding?.target}</div>
                      <div className="mt-1 text-[10px] text-muted-foreground">
                        {relativeTime(item.finding?.createdAt)} · {item.occurrences || 1} 次
                      </div>
                    </div>
                  ))}
                </div>
              ) : (
                <div className="py-10 text-center text-xs text-muted-foreground">当前时间范围内暂无风险</div>
              )}
            </CardContent>
          </Card>
          <Card>
            <CardHeader className="border-b py-3">
              <CardTitle>风险分布</CardTitle>
            </CardHeader>
            <CardContent className="space-y-2 p-3">
              {(['critical', 'high', 'medium', 'low', 'info'] as const).map((key) => (
                <div key={key} className="flex items-center gap-2 text-xs">
                  <span className={`w-10 shrink-0 risk-${key}`}>{riskLabel(key)}</span>
                  <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-muted">
                    <div
                      className={`h-full risk-bg-${key}`}
                      style={{
                        width: `${Math.min(100, ((data.severity[key] || 0) / Math.max(1, data.summary.currentFindings || 1)) * 100)}%`,
                      }}
                    />
                  </div>
                  <span className="w-7 text-right tabular-nums">{data.severity[key] || 0}</span>
                </div>
              ))}
            </CardContent>
          </Card>
        </div>
      </div>
      {data.truncated ? (
        <div className="mt-3 flex items-center gap-2 rounded-md border bg-card px-3 py-2 text-[11px] text-muted-foreground">
          <AlertTriangle className="size-3.5" />
          数据量超过当前展示上限，部分组织未出现在排行和画布中。
        </div>
      ) : null}
    </div>
  )
}

function HealthChip({ label, value }: { label: string; value?: string }) {
  const ok = value === 'ok'
  return (
    <span className="inline-flex items-center gap-1 rounded-full border bg-card px-2 py-1 text-[10px] text-muted-foreground">
      <span className={cn('size-1.5 rounded-full', ok ? 'bg-[#12b76a]' : 'bg-[#f79009]')} />
      {label}
    </span>
  )
}
function MiniFact({ label, value }: { label: string; value: number }) {
  return (
    <div className="rounded-md bg-muted px-2.5 py-2">
      <div className="text-[10px] text-muted-foreground">{label}</div>
      <div className="mt-0.5 text-base font-semibold tabular-nums">{value}</div>
    </div>
  )
}
