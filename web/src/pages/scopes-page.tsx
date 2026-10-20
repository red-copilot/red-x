import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Boxes, Plus, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { api, type Scope } from '@/api'
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
import { Textarea } from '@/components/ui/textarea'
import { canWrite, isAdmin } from '@/lib/access'
import { useAuth } from '@/app/auth'
import { formatTime } from '@/lib/format'

export function ScopesPage() {
  const { role } = useAuth()
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)
  const [form, setForm] = useState({ name: '', type: 'domain', targets: '' })
  const scopes = useQuery({ queryKey: ['scopes'], queryFn: () => api.scopes() })
  const refresh = () => queryClient.invalidateQueries({ queryKey: ['scopes'] })
  const create = useMutation({
    mutationFn: () => api.createScope({ ...form, targets: form.targets.split(/[\s,]+/).filter(Boolean) }),
    onSuccess: () => {
      toast.success('资产组已创建')
      setOpen(false)
      setForm({ name: '', type: 'domain', targets: '' })
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
  const rows = (scopes.data?.items || []) as Scope[]
  return (
    <div>
      <PageHeader
        title="资产组"
        description="兼容原有按域名或 IP 范围聚合资产的工作方式。"
        actions={
          canWrite(role) ? (
            <Button onClick={() => setOpen(true)}>
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
            <DialogTitle>新建资产组</DialogTitle>
            <DialogDescription>范围支持逗号、空格或换行分隔。</DialogDescription>
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
            <Field label="范围">
              <Textarea value={form.targets} onChange={(e) => setForm({ ...form, targets: e.target.value })} />
            </Field>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setOpen(false)}>
              取消
            </Button>
            <Button disabled={!form.name.trim() || !form.targets.trim()} onClick={() => create.mutate()}>
              创建资产组
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
