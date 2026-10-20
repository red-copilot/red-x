import { AlertTriangle, Inbox, LoaderCircle } from 'lucide-react'
import { Button } from '@/components/ui/button'

export function LoadingBlock({ label = '正在加载…' }: { label?: string }) {
  return (
    <div className="grid min-h-40 place-items-center rounded-lg border bg-card">
      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <LoaderCircle className="size-4 animate-spin" />
        {label}
      </div>
    </div>
  )
}

export function EmptyState({
  title,
  description,
  action,
}: {
  title: string
  description?: string
  action?: React.ReactNode
}) {
  return (
    <div className="grid min-h-48 place-items-center rounded-lg border border-dashed bg-card px-6 text-center">
      <div>
        <Inbox className="mx-auto mb-3 size-7 text-muted-foreground" />
        <div className="text-sm font-medium">{title}</div>
        {description ? <p className="mt-1 max-w-md text-xs leading-5 text-muted-foreground">{description}</p> : null}
        {action ? <div className="mt-4">{action}</div> : null}
      </div>
    </div>
  )
}

export function ErrorState({ error, retry }: { error: unknown; retry?: () => void }) {
  const message = error instanceof Error ? error.message : '请求失败，请稍后重试'
  return (
    <div className="rounded-lg border border-[#f1b8b2] bg-[#fff8f7] p-4">
      <div className="flex items-start gap-3">
        <AlertTriangle className="mt-0.5 size-4 text-destructive" />
        <div className="min-w-0 flex-1">
          <div className="text-sm font-medium text-[#8f1c13]">数据加载失败</div>
          <p className="mt-1 break-words text-xs leading-5 text-[#7a302a]">{message}</p>
        </div>
        {retry ? (
          <Button size="sm" variant="outline" onClick={retry}>
            重试
          </Button>
        ) : null}
      </div>
    </div>
  )
}
