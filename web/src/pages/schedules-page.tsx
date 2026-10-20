import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { CalendarClock, Pencil, Plus, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { api, type Schedule } from '@/api'
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

export function SchedulesPage() {
  const { role } = useAuth()
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)
  const [editingId, setEditingId] = useState('')
  const [form, setForm] = useState({
    name: '',
    kind: 'recurrent_scan',
    taskKind: 'domain',
    target: '',
    policyId: '',
    cron: '0 2 * * *',
    startAt: '',
  })
  const schedules = useQuery({ queryKey: ['schedules'], queryFn: () => api.schedules() })
  const policies = useQuery({ queryKey: ['policies'], queryFn: () => api.policies() })
  const refresh = () => queryClient.invalidateQueries({ queryKey: ['schedules'] })
  const create = useMutation({
    mutationFn: () =>
      api.createSchedule({ ...form, startAt: form.startAt ? new Date(form.startAt).toISOString() : undefined }),
    onSuccess: () => {
      toast.success('计划任务已创建')
      setOpen(false)
      void refresh()
    },
    onError: (error) => toast.error(error.message),
  })
  const remove = useMutation({
    mutationFn: (id: string) => api.deleteSchedule(id),
    onSuccess: () => {
      toast.success('计划已删除')
      void refresh()
    },
    onError: (error) => toast.error(error.message),
  })
  const update = useMutation({
    mutationFn: () =>
      api.updateSchedule(editingId, {
        ...form,
        startAt: form.startAt ? new Date(form.startAt).toISOString() : undefined,
      }),
    onSuccess: () => {
      toast.success('计划任务已更新')
      setOpen(false)
      setEditingId('')
      void refresh()
    },
    onError: (error) => toast.error(error.message),
  })
  function edit(row: Schedule) {
    setEditingId(row.id || '')
    setForm({
      name: row.name || '',
      kind: String(row.kind || 'recurrent_scan'),
      taskKind: String(row.taskKind || 'domain'),
      target: String(row.target || ''),
      policyId: String(row.policyId || ''),
      cron: String(row.cron || ''),
      startAt: row.startAt ? new Date(String(row.startAt)).toISOString().slice(0, 16) : '',
    })
    setOpen(true)
  }
  const rows = (schedules.data?.items || []) as Schedule[]
  const policyRows = (policies.data?.items || []) as Array<{ id?: string; name?: string }>
  return (
    <div>
      <PageHeader
        title="计划任务"
        description="维护 Cron 周期扫描与未来一次性任务。"
        actions={
          canWrite(role) ? (
            <Button
              onClick={() => {
                setEditingId('')
                setForm({
                  name: '',
                  kind: 'recurrent_scan',
                  taskKind: 'domain',
                  target: '',
                  policyId: '',
                  cron: '0 2 * * *',
                  startAt: '',
                })
                setOpen(true)
              }}
            >
              <Plus />
              新建计划
            </Button>
          ) : null
        }
      />
      {schedules.isLoading ? (
        <LoadingBlock />
      ) : schedules.error ? (
        <ErrorState error={schedules.error} retry={() => schedules.refetch()} />
      ) : (
        <DataTable
          rows={rows}
          rowKey={(row) => row.id || ''}
          empty={
            <EmptyState
              title="暂无计划任务"
              action={<CalendarClock className="mx-auto mt-2 size-5 text-muted-foreground" />}
            />
          }
          columns={[
            { key: 'name', header: '名称', cell: (row) => row.name },
            { key: 'kind', header: '类型', cell: (row) => String(row.kind || '—') },
            { key: 'target', header: '目标', cell: (row) => <span className="mono">{String(row.target || '—')}</span> },
            {
              key: 'cron',
              header: 'Cron',
              cell: (row) => <span className="mono">{row.cron || '—'}</span>,
              hideOnTablet: true,
            },
            { key: 'status', header: '状态', cell: (row) => <StatusBadge status={String(row.status || '')} /> },
            { key: 'next', header: '下次运行', cell: (row) => formatTime(row.nextRunAt) },
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
                    <ConfirmDialog
                      title="删除计划任务"
                      description="计划会移入回收站并停止后续运行，历史扫描任务会保留。"
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
            <DialogTitle>{editingId ? '编辑计划任务' : '新建计划任务'}</DialogTitle>
            <DialogDescription>计划必须绑定有效策略；保存后重新计算下次运行时间。</DialogDescription>
          </DialogHeader>
          <div className="space-y-3">
            <Field label="名称">
              <Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
            </Field>
            <div className="grid gap-3 sm:grid-cols-2">
              <Field label="计划类型">
                <Select value={form.kind} onValueChange={(value) => setForm({ ...form, kind: value })}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="recurrent_scan">周期扫描</SelectItem>
                    <SelectItem value="future_scan">一次性任务</SelectItem>
                  </SelectContent>
                </Select>
              </Field>
              <Field label="任务类型">
                <Select value={form.taskKind} onValueChange={(value) => setForm({ ...form, taskKind: value })}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="domain">域名</SelectItem>
                    <SelectItem value="ip">IP</SelectItem>
                  </SelectContent>
                </Select>
              </Field>
            </div>
            <Field label="目标">
              <Input value={form.target} onChange={(e) => setForm({ ...form, target: e.target.value })} />
            </Field>
            <Field label="扫描策略">
              <Select value={form.policyId} onValueChange={(value) => setForm({ ...form, policyId: value })}>
                <SelectTrigger>
                  <SelectValue placeholder="选择扫描策略" />
                </SelectTrigger>
                <SelectContent>
                  {policyRows.map((policy) => (
                    <SelectItem key={policy.id || ''} value={policy.id || ''}>
                      {policy.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </Field>
            {form.kind === 'recurrent_scan' ? (
              <Field label="Cron">
                <Input value={form.cron} onChange={(e) => setForm({ ...form, cron: e.target.value })} />
              </Field>
            ) : (
              <Field label="开始时间">
                <Input
                  type="datetime-local"
                  value={form.startAt}
                  onChange={(e) => setForm({ ...form, startAt: e.target.value })}
                />
              </Field>
            )}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setOpen(false)}>
              取消
            </Button>
            <Button
              disabled={!form.name.trim() || !form.target.trim() || !form.policyId || create.isPending || update.isPending}
              onClick={() => (editingId ? update.mutate() : create.mutate())}
            >
              {editingId ? '保存修改' : '创建计划'}
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
