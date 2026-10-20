import { useQuery } from '@tanstack/react-query'
import { Building2, Crosshair, FileWarning, Globe2, Search, Workflow } from 'lucide-react'
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '@/api'
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command'
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '@/components/ui/dialog'

const icons = { organization: Building2, domain: Globe2, asset: Crosshair, finding: FileWarning, task: Workflow }
const labels = { organizations: '组织', domains: '域名', assets: '资产', findings: '风险', tasks: '任务' }

function targetPath(type: string, id: string, organizationId?: string) {
  if (type === 'organization') return `/organizations/${id}`
  if (type === 'finding')
    return organizationId ? `/organizations/${organizationId}/findings?focus=${id}` : `/findings?focus=${id}`
  if (type === 'task') return `/tasks/${id}`
  if (organizationId) return `/organizations/${organizationId}/assets?focus=${id}`
  return `/assets?focus=${id}`
}

export function CommandPalette({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  const [query, setQuery] = useState('')
  const navigate = useNavigate()
  const search = useQuery({
    queryKey: ['search', query],
    queryFn: () => api.search(query),
    enabled: open && query.trim().length >= 2,
    staleTime: 10_000,
  })
  const groups = (['organizations', 'domains', 'assets', 'findings', 'tasks'] as const).map((key) => ({
    key,
    items: search.data?.[key] || [],
  }))

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="top-20 max-w-2xl translate-y-0 p-0">
        <DialogTitle className="sr-only">全局搜索</DialogTitle>
        <DialogDescription className="sr-only">搜索组织、域名、资产、风险和任务</DialogDescription>
        <Command shouldFilter={false} loop>
          <CommandInput value={query} onValueChange={setQuery} placeholder="搜索组织、域名、资产、风险或任务…" />
          <CommandList>
            {query.trim().length < 2 ? (
              <div className="flex items-center gap-2 px-3 py-8 text-xs text-muted-foreground">
                <Search className="size-4" />
                输入至少 2 个字符开始搜索
              </div>
            ) : search.isLoading ? (
              <div className="px-3 py-8 text-center text-xs text-muted-foreground">正在搜索…</div>
            ) : (
              <CommandEmpty>没有匹配结果</CommandEmpty>
            )}
            {groups.map(({ key, items }) => {
              if (!items.length) return null
              return (
                <CommandGroup key={key} heading={labels[key]}>
                  {items.map((item) => {
                    const Icon = icons[item.type as keyof typeof icons] || Search
                    return (
                      <CommandItem
                        key={`${key}-${item.id}`}
                        value={`${key}-${item.id}`}
                        onSelect={() => {
                          onOpenChange(false)
                          navigate(targetPath(item.type, item.id, item.organizationId))
                        }}
                      >
                        <Icon className="size-4 text-muted-foreground" />
                        <div className="min-w-0 flex-1">
                          <div className="truncate font-medium">{item.label}</div>
                          {item.detail ? (
                            <div className="truncate text-[11px] text-muted-foreground">{item.detail}</div>
                          ) : null}
                        </div>
                        {item.risk ? <span className={`text-[11px] risk-${item.risk}`}>{item.risk}</span> : null}
                      </CommandItem>
                    )
                  })}
                </CommandGroup>
              )
            })}
          </CommandList>
        </Command>
      </DialogContent>
    </Dialog>
  )
}
