import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Activity, Pause, Play, Plus, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { api, type Monitor } from '@/api'
import { StatusBadge } from '@/components/common/badges'
import { DataTable } from '@/components/common/data-table'
import { PageHeader } from '@/components/common/page-header'
import { EmptyState, ErrorState, LoadingBlock } from '@/components/common/states'
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
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { formatTime } from '@/lib/format'
import { canWrite } from '@/lib/access'
import { useAuth } from '@/app/auth'

export function MonitorsPage() {
  const { role } = useAuth()
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)
  const [form, setForm] = useState({ name: '', kind: 'domain', target: '', intervalSec: 21600, policyId: '' })
  const monitors = useQuery({ queryKey: ['monitors'], queryFn: () => api.monitors() })
  const refresh = () => queryClient.invalidateQueries({ queryKey: ['monitors'] })
  const create = useMutation({
    mutationFn: () => api.createMonitor(form),
    onSuccess: () => {
      toast.success('监控已创建')
      setOpen(false)
      void refresh()
    },
    onError: (error) => toast.error(error.message),
  })
  const action = useMutation({
    mutationFn: ({ id, action }: { id: string; action: 'pause' | 'resume' | 'run' }) => api.monitorAction(id, action),
    onSuccess: (_, vars) => {
      toast.success(vars.action === 'run' ? '已触发运行' : '状态已更新')
      void refresh()
    },
    onError: (error) => toast.error(error.message),
  })
  const remove = useMutation({
    mutationFn: (id: string) => api.deleteMonitor(id),
    onSuccess: () => {
      toast.success('监控已删除')
      void refresh()
    },
    onError: (error) => toast.error(error.message),
  })
  const rows = (monitors.data?.items || []) as Monitor[]
  return (
    <div>
      <PageHeader
        title="周期监控"
        description="对域名、IP、站点或 WIH 变化建立周期任务。"
        actions={
          canWrite(role) ? (
            <Button onClick={() => setOpen(true)}>
              <Plus />
              新建监控
            </Button>
          ) : null
        }
      />
      {monitors.isLoading ? (
        <LoadingBlock />
      ) : monitors.error ? (
        <ErrorState error={monitors.error} retry={() => monitors.refetch()} />
      ) : (
        <DataTable
          rows={rows}
          rowKey={(row) => row.id || ''}
          empty={
            <EmptyState title="暂无监控" action={<Activity className="mx-auto mt-2 size-5 text-muted-foreground" />} />
          }
          columns={[
            { key: 'name', header: '名称', cell: (row) => row.name },
            { key: 'kind', header: '类型', cell: (row) => row.kind },
            { key: 'target', header: '目标', cell: (row) => <span className="mono">{row.target}</span> },
            { key: 'status', header: '状态', cell: (row) => <StatusBadge status={row.status} /> },
            { key: 'next', header: '下次运行', cell: (row) => formatTime(row.nextRunAt), hideOnTablet: true },
            {
              key: 'actions',
              header: '操作',
              cell: (row) =>
                canWrite(role) ? (
                  <div className="flex justify-end gap-1">
                    <Button
                      size="sm"
                      variant="ghost"
                      onClick={() => action.mutate({ id: row.id || '', action: 'run' })}
                    >
                      <Play />
                      运行
                    </Button>
                    <Button
                      size="sm"
                      variant="ghost"
                      onClick={() =>
                        action.mutate({ id: row.id || '', action: row.status === 'active' ? 'pause' : 'resume' })
                      }
                    >
                      {row.status === 'active' ? (
                        <>
                          <Pause />
                          暂停
                        </>
                      ) : (
                        <>
                          <Play />
                          恢复
                        </>
                      )}
                    </Button>
                    <Button
                      size="sm"
                      variant="ghost"
                      className="text-destructive"
                      onClick={() => remove.mutate(row.id || '')}
                    >
                      <Trash2 />
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
            <DialogTitle>新建监控</DialogTitle>
            <DialogDescription>周期检查结果会创建后续任务并保留变更历史。</DialogDescription>
          </DialogHeader>
          <div className="space-y-3">
            <Field label="名称">
              <Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
            </Field>
            <div className="grid gap-3 sm:grid-cols-2">
              <Field label="类型">
                <Select value={form.kind} onValueChange={(value) => setForm({ ...form, kind: value })}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {['domain', 'ip', 'site_update', 'wih_update'].map((item) => (
                      <SelectItem key={item} value={item}>
                        {item}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </Field>
              <Field label="间隔（秒）">
                <Input
                  type="number"
                  value={form.intervalSec}
                  onChange={(e) => setForm({ ...form, intervalSec: Number(e.target.value) })}
                />
              </Field>
            </div>
            <Field label="目标">
              <Input value={form.target} onChange={(e) => setForm({ ...form, target: e.target.value })} />
            </Field>
            <Field label="策略 ID（可选）">
              <Input value={form.policyId} onChange={(e) => setForm({ ...form, policyId: e.target.value })} />
            </Field>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setOpen(false)}>
              取消
            </Button>
            <Button
              disabled={!form.name.trim() || !form.target.trim() || create.isPending}
              onClick={() => create.mutate()}
            >
              创建监控
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
