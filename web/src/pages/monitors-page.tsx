import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Activity, Pause, Pencil, Play, Plus, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { api, type Monitor } from '@/api'
import { StatusBadge } from '@/components/common/badges'
import { DataTable } from '@/components/common/data-table'
import { PageHeader } from '@/components/common/page-header'
import { EmptyState, ErrorState, LoadingBlock } from '@/components/common/states'
import { ConfirmDialog } from '@/components/common/confirm-dialog'
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
  const [editingId, setEditingId] = useState('')
  const [form, setForm] = useState({
    name: '',
    kind: 'domain',
    target: '',
    intervalSec: 21600,
    scopeId: '',
    policyId: '',
  })
  const monitors = useQuery({ queryKey: ['monitors'], queryFn: () => api.monitors() })
  const scopes = useQuery({ queryKey: ['scopes'], queryFn: () => api.scopes() })
  const policies = useQuery({ queryKey: ['policies'], queryFn: () => api.policies() })
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
  const update = useMutation({
    mutationFn: () => api.updateMonitor(editingId, form),
    onSuccess: () => {
      toast.success('监控已更新')
      setOpen(false)
      setEditingId('')
      void refresh()
    },
    onError: (error) => toast.error(error.message),
  })
  function edit(row: Monitor) {
    setEditingId(row.id || '')
    setForm({
      name: row.name || '',
      kind: row.kind || 'domain',
      target: row.target || '',
      intervalSec: row.intervalSec || 21600,
      scopeId: row.scopeId || '',
      policyId: row.policyId || '',
    })
    setOpen(true)
  }
  const rows = (monitors.data?.items || []) as Monitor[]
  const scopeRows = (scopes.data?.items || []) as Array<{ id?: string; name?: string }>
  const policyRows = (policies.data?.items || []) as Array<{ id?: string; name?: string }>
  return (
    <div>
      <PageHeader
        title="周期监控"
        description="对域名、IP、站点或 WIH 变化建立周期任务。"
        actions={
          canWrite(role) ? (
            <Button
              onClick={() => {
                setEditingId('')
                setForm({
                  name: '',
                  kind: 'domain',
                  target: '',
                  intervalSec: 21600,
                  scopeId: '',
                  policyId: '',
                })
                setOpen(true)
              }}
            >
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
                    <Button size="sm" variant="ghost" onClick={() => edit(row)}>
                      <Pencil />
                      编辑
                    </Button>
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
                    <ConfirmDialog
                      title="删除监控"
                      description="监控会移入回收站并停止后续调度，历史运行记录会保留。"
                      confirmLabel="移入回收站"
                      onConfirm={() => remove.mutate(row.id || '')}
                      trigger={
                        <Button size="sm" variant="ghost" className="text-destructive">
                          <Trash2 />
                          删除
                        </Button>
                      }
                    />
                  </div>
                ) : null,
            },
          ]}
        />
      )}
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{editingId ? '编辑监控' : '新建监控'}</DialogTitle>
            <DialogDescription>监控必须绑定有效资产组；编辑后将从新间隔重新计算下次运行时间。</DialogDescription>
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
            <Field label="资产组">
              <Select value={form.scopeId} onValueChange={(value) => setForm({ ...form, scopeId: value })}>
                <SelectTrigger>
                  <SelectValue placeholder="选择资产组" />
                </SelectTrigger>
                <SelectContent>
                  {scopeRows.map((scope) => (
                    <SelectItem key={scope.id || ''} value={scope.id || ''}>
                      {scope.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </Field>
            <Field label="策略 ID（可选）">
              <Select
                value={form.policyId || 'none'}
                onValueChange={(value) => setForm({ ...form, policyId: value === 'none' ? '' : value })}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="none">使用任务默认策略</SelectItem>
                  {policyRows.map((policy) => (
                    <SelectItem key={policy.id || ''} value={policy.id || ''}>
                      {policy.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </Field>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setOpen(false)}>
              取消
            </Button>
            <Button
              disabled={
                !form.name.trim() || !form.target.trim() || !form.scopeId || create.isPending || update.isPending
              }
              onClick={() => (editingId ? update.mutate() : create.mutate())}
            >
              {editingId ? '保存修改' : '创建监控'}
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
