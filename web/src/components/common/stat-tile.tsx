import type { ReactNode } from 'react'
import { Card } from '@/components/ui/card'

export function StatTile({
  label,
  value,
  hint,
  icon,
  testId,
}: {
  label: string
  value: ReactNode
  hint?: ReactNode
  icon?: ReactNode
  testId?: string
}) {
  return (
    <Card className="min-w-0 p-4" data-testid={testId}>
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="text-[11px] font-medium uppercase tracking-[0.08em] text-muted-foreground">{label}</div>
          <div className="mt-2 text-2xl font-semibold tabular-nums tracking-tight">{value}</div>
          {hint ? <div className="mt-1 truncate text-[11px] text-muted-foreground">{hint}</div> : null}
        </div>
        {icon ? <div className="rounded-md bg-muted p-2 text-muted-foreground">{icon}</div> : null}
      </div>
    </Card>
  )
}
