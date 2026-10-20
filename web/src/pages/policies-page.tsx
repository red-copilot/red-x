import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Plus, Settings2, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { api, type Policy } from '@/api'
import { DataTable } from '@/components/common/data-table'
import { PageHeader } from '@/components/common/page-header'
import { EmptyState, ErrorState, LoadingBlock } from '@/components/common/states'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
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
import { Textarea } from '@/components/ui/textarea'
import { policySummary } from '@/lib/format'
import { canWrite, isAdmin } from '@/lib/access'
import { useAuth } from '@/app/auth'

const defaults = {
  domainBrute: true,
  portScan: true,
  serviceDetection: true,
  siteIdentify: true,
  fileLeak: false,
  webInfoHunter: false,
  nucleiScan: false,
}
export function PoliciesPage() {
  const { role } = useAuth()
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [options, setOptions] = useState(defaults)
  const policies = useQuery({ queryKey: ['policies'], queryFn: () => api.policies() })
  const refresh = () => queryClient.invalidateQueries({ queryKey: ['policies'] })
  const create = useMutation({
    mutationFn: () =>
      api.createPolicy({
        name,
        description,
        config: {
          domain: { domainBrute: options.domainBrute },
          ip: { portScan: options.portScan, serviceDetection: options.serviceDetection },
          site: {
            siteIdentify: options.siteIdentify,
            fileLeak: options.fileLeak,
            webInfoHunter: options.webInfoHunter,
            nucleiScan: options.nucleiScan,
          },
          poc: [],
          brute: [],
        },
      }),
    onSuccess: () => {
      toast.success('策略已创建')
      setOpen(false)
      setName('')
      setDescription('')
      void refresh()
    },
    onError: (error) => toast.error(error.message),
  })
  const remove = useMutation({
    mutationFn: (id: string) => api.deletePolicy(id),
    onSuccess: () => {
      toast.success('策略已删除')
      void refresh()
    },
    onError: (error) => toast.error(error.message),
  })
  const rows = (policies.data?.items || []) as Policy[]
  return (
    <div>
      <PageHeader
        title="扫描策略"
        description="组合域名、IP、站点与 PoC 能力，供任务、计划和监控复用。"
        actions={
          canWrite(role) ? (
            <Button onClick={() => setOpen(true)}>
              <Plus />
              新建策略
            </Button>
          ) : null
        }
      />
      {policies.isLoading ? (
        <LoadingBlock />
      ) : policies.error ? (
        <ErrorState error={policies.error} retry={() => policies.refetch()} />
      ) : (
        <DataTable
          rows={rows}
          rowKey={(row) => row.id || row.name || ''}
          empty={
            <EmptyState
              title="暂无扫描策略"
              action={<Settings2 className="mx-auto mt-2 size-5 text-muted-foreground" />}
            />
          }
          columns={[
            { key: 'name', header: '名称', cell: (row) => <span className="font-medium">{row.name}</span> },
            { key: 'description', header: '描述', cell: (row) => row.description || '—', hideOnTablet: true },
            {
              key: 'config',
              header: '启用能力',
              cell: (row) => (
                <span className="text-muted-foreground">{policySummary(row.config as Record<string, any>)}</span>
              ),
            },
            {
              key: 'actions',
              header: '操作',
              cell: (row) =>
                isAdmin(role) ? (
                  <div className="flex justify-end">
                    <Button
                      size="sm"
                      variant="ghost"
                      className="text-destructive"
                      onClick={() => remove.mutate(row.id || '')}
                    >
                      <Trash2 />
                      删除
                    </Button>
                  </div>
                ) : null,
            },
          ]}
        />
      )}
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>新建扫描策略</DialogTitle>
            <DialogDescription>默认启用域名枚举、端口与服务识别、站点指纹。</DialogDescription>
          </DialogHeader>
          <div className="space-y-3">
            <Field label="名称">
              <Input value={name} onChange={(e) => setName(e.target.value)} />
            </Field>
            <Field label="描述">
              <Textarea value={description} onChange={(e) => setDescription(e.target.value)} className="min-h-16" />
            </Field>
            <div className="grid grid-cols-2 gap-2">
              {Object.entries(options).map(([key, checked]) => (
                <label key={key} className="flex items-center gap-2 rounded-md border px-3 py-2 text-xs">
                  <Checkbox
                    checked={checked}
                    onCheckedChange={(value) => setOptions({ ...options, [key]: Boolean(value) })}
                  />
                  {key}
                </label>
              ))}
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setOpen(false)}>
              取消
            </Button>
            <Button disabled={!name.trim()} onClick={() => create.mutate()}>
              创建策略
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
