import { useQuery } from '@tanstack/react-query'
import { ExternalLink } from 'lucide-react'
import { api, type SystemInfo } from '@/api'
import { PageHeader } from '@/components/common/page-header'
import { ErrorState, LoadingBlock } from '@/components/common/states'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'

export function SystemPage() {
  const info = useQuery({ queryKey: ['system-info'], queryFn: () => api.systemInfo(), refetchInterval: 30_000 })
  const ready = useQuery({ queryKey: ['readyz'], queryFn: () => api.ready(), refetchInterval: 15_000 })
  if (info.isLoading) return <LoadingBlock />
  if (info.error) return <ErrorState error={info.error} retry={() => info.refetch()} />
  const data = info.data as SystemInfo
  return (
    <div>
      <PageHeader title="系统" description="运行状态、扫描器版本、可观测性与接口文档。" />
      <div className="grid gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>运行信息</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2 text-xs">
            <Row label="版本" value={String(data.version || '—')} />
            <Row label="运行时长" value={`${Number(data.uptimeSec || 0)} 秒`} />
            <Row label="队列驱动" value={String(data.queueDriver || '—')} />
            <Row
              label="健康"
              value={ready.data ? <Badge variant="success">ready</Badge> : <Badge variant="warning">degraded</Badge>}
            />
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>接口与监控</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2 text-xs">
            {[
              'API 文档|/api/v1/docs',
              'OpenAPI 3.1|/api/v1/openapi.json',
              'Prometheus Metrics|/metrics',
              'Readyz|/readyz',
            ].map((item) => {
              const [label, href] = item.split('|')
              return (
                <a
                  key={href}
                  href={href}
                  target="_blank"
                  rel="noreferrer"
                  className="flex items-center justify-between rounded-md border px-3 py-2 hover:bg-muted"
                >
                  <span>{label}</span>
                  <ExternalLink className="size-3.5 text-muted-foreground" />
                </a>
              )
            })}
          </CardContent>
        </Card>
        <Card className="lg:col-span-2">
          <CardHeader>
            <CardTitle>扫描器 SDK</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-4">
              {Object.entries(data.scanners || {}).map(([name, version]) => (
                <div key={name} className="rounded-md border bg-[#fbfcfe] p-3">
                  <div className="text-[10px] uppercase tracking-wider text-muted-foreground">{name}</div>
                  <div className="mono mt-1 text-xs font-medium">{String(version)}</div>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between border-b pb-2 last:border-0">
      <span className="text-muted-foreground">{label}</span>
      <span className="font-medium">{value}</span>
    </div>
  )
}
