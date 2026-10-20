import { Badge } from '@/components/ui/badge'
import { riskLabel, statusLabel } from '@/lib/format'

export function RiskBadge({ risk }: { risk?: string }) {
  const variant =
    risk === 'critical' || risk === 'high'
      ? 'danger'
      : risk === 'medium'
        ? 'warning'
        : risk === 'low'
          ? 'info'
          : 'outline'
  return <Badge variant={variant}>{riskLabel(risk)}</Badge>
}

export function StatusBadge({ status }: { status?: string }) {
  const variant =
    status === 'succeeded' || status === 'active' || status === 'confirmed' || status === 'done'
      ? 'success'
      : status === 'running' || status === 'queued'
        ? 'info'
        : status === 'failed' || status === 'rejected'
          ? 'danger'
          : status === 'partial' || status === 'paused'
            ? 'warning'
            : 'outline'
  return <Badge variant={variant}>{statusLabel(status)}</Badge>
}
