import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Boxes, Pencil, Plus, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { api, type Scope } from '@/api'
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
import { Textarea } from '@/components/ui/textarea'
import { canWrite } from '@/lib/access'
import { useAuth } from '@/app/auth'
import { formatTime } from '@/lib/format'

export function ScopesPage() {
  const { role } = useAuth()
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)
  const [editingId, setEditingId] = useState('')
  const [form, setForm] = useState({ name: '', type: 'domain', targets: '', blackScope: '' })
  const scopes = useQuery({ queryKey: ['scopes'], queryFn: () => api.scopes() })
  const refresh = () => queryClient.invalidateQueries({ queryKey: ['scopes'] })
  const create = useMutation({
    mutationFn: () =>
      api.createScope({
        ...form,
        targets: form.targets.split(/[\s,]+/).filter(Boolean),
        blackScope: form.blackScope.split(/[\s,]+/).filter(Boolean),
      }),
    onSuccess: () => {
      toast.success('资产组已创建')
      setOpen(false)
      setForm({ name: '', type: 'domain', targets: '', blackScope: '' })
      void refresh()
    },
    onError: (error) => toast.error(error.message),
  })
  const remove = useMutation({
    mutationFn: (id: string) => api.deleteScope(id),
    onSuccess: () => {
      toast.success('资产组已删除')
      void refresh()
    },
    onError: (error) => toast.error(error.message),
  })
  const update = useMutation({
    mutationFn: () =>
      api.updateScope(editingId, {
        ...form,
        targets: form.targets.split(/[\s,]+/).filter(Boolean),
        blackScope: form.blackScope.split(/[\s,]+/).filter(Boolean),
      }),
    onSuccess: () => {
      toast.success('资产组已更新')
      setOpen(false)
      setEditingId('')
      void refresh()
    },
    onError: (error) => toast.error(error.message),
  })
  function edit(row: Scope) {
    setEditingId(row.id || '')
    setForm({
      name: row.name || '',
      type: row.type || 'domain',
      targets: (row.targets || []).join('\n'),
      blackScope: (row.blackScope || []).join('\n'),
    })
    setOpen(true)
  }
  const rows = (scopes.data?.items || []) as Scope[]
  return (
    <div>
      <PageHeader
        title="资产组"
        description="定义任务授权白名单；声明 Scope 的任务只能扫描其域名或 IP/CIDR 范围，黑名单优先。"
        actions={
          canWrite(role) ? (
            <Button
              onClick={() => {
                setEditingId('')
                setForm({ name: '', type: 'domain', targets: '', blackScope: '' })
                setOpen(true)
              }}
            >
              <Plus />
              新建资产组
            </Button>
          ) : null
        }
      />
      {scopes.isLoading ? (
        <LoadingBlock />
      ) : scopes.error ? (
        <ErrorState error={scopes.error} retry={() => scopes.refetch()} />
      ) : (
        <DataTable
          rows={rows}
          rowKey={(row) => row.id || row.name || ''}
          empty={
            <EmptyState title="暂无资产组" action={<Boxes className="mx-auto mt-2 size-5 text-muted-foreground" />} />
          }
          columns={[
            { key: 'name', header: '名称', cell: (row) => row.name },
            { key: 'type', header: '类型', cell: (row) => row.type },
            {
              key: 'targets',
              header: '范围',
              cell: (row) => <span className="mono max-w-[420px] truncate">{row.targets?.join(', ') || '—'}</span>,
            },
            { key: 'created', header: '创建时间', cell: (row) => formatTime(row.createdAt), hideOnTablet: true },
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
                      title="删除资产组"
                      description="资产组会移入回收站；仍被活动监控引用时无法删除。"
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
            <DialogTitle>{editingId ? '编辑资产组' : '新建资产组'}</DialogTitle>
            <DialogDescription>
              {editingId ? '范围修改只影响后续任务和监控，不会重建历史关联。' : '范围支持逗号、空格或换行分隔。'}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-3">
            <Field label="名称">
              <Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
            </Field>
            <Field label="类型">
              <Select value={form.type} onValueChange={(value) => setForm({ ...form, type: value })}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="domain">域名</SelectItem>
                  <SelectItem value="ip">IP / CIDR</SelectItem>
                </SelectContent>
              </Select>
            </Field>
            <Field label="授权范围">
              <Textarea value={form.targets} onChange={(e) => setForm({ ...form, targets: e.target.value })} />
            </Field>
            <Field label="排除范围">
              <Textarea
                value={form.blackScope}
                placeholder="留空表示不额外排除"
                onChange={(e) => setForm({ ...form, blackScope: e.target.value })}
              />
            </Field>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setOpen(false)}>
              取消
            </Button>
            <Button
              disabled={!form.name.trim() || !form.targets.trim() || create.isPending || update.isPending}
              onClick={() => (editingId ? update.mutate() : create.mutate())}
            >
              {editingId ? '保存修改' : '创建资产组'}
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
