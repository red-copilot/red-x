import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Plus, Trash2, Users } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { api, type User } from '@/api'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { DataTable } from '@/components/common/data-table'
import { PageHeader } from '@/components/common/page-header'
import { EmptyState, ErrorState, LoadingBlock } from '@/components/common/states'
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
import { formatTime, roleLabel } from '@/lib/format'

export function UsersPage() {
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)
  const [form, setForm] = useState({ username: '', password: '', role: 'viewer' })
  const users = useQuery({ queryKey: ['users'], queryFn: () => api.users() })
  const refresh = () => queryClient.invalidateQueries({ queryKey: ['users'] })
  const create = useMutation({
    mutationFn: () => api.createUser(form),
    onSuccess: () => {
      toast.success('用户已创建')
      setOpen(false)
      setForm({ username: '', password: '', role: 'viewer' })
      void refresh()
    },
    onError: (error) => toast.error(error.message),
  })
  const remove = useMutation({
    mutationFn: (id: string) => api.deleteUser(id),
    onSuccess: () => {
      toast.success('用户已删除')
      void refresh()
    },
    onError: (error) => toast.error(error.message),
  })
  const rows = (users.data?.items || []) as User[]
  return (
    <div>
      <PageHeader
        title="用户管理"
        description="维护 viewer、operator 与 admin 账户。初始密码建议要求用户首次登录后修改。"
        actions={
          <Button onClick={() => setOpen(true)}>
            <Plus />
            新建用户
          </Button>
        }
      />
      {users.isLoading ? (
        <LoadingBlock />
      ) : users.error ? (
        <ErrorState error={users.error} retry={() => users.refetch()} />
      ) : (
        <DataTable
          rows={rows}
          rowKey={(row) => row.id || row.username || ''}
          empty={
            <EmptyState title="暂无用户" action={<Users className="mx-auto mt-2 size-5 text-muted-foreground" />} />
          }
          columns={[
            { key: 'username', header: '用户名', cell: (row) => <span className="font-medium">{row.username}</span> },
            {
              key: 'role',
              header: '角色',
              cell: (row) => (
                <Badge variant={row.role === 'admin' ? 'danger' : row.role === 'operator' ? 'info' : 'outline'}>
                  {roleLabel(row.role)}
                </Badge>
              ),
            },
            { key: 'mustChange', header: '首次改密', cell: (row) => (row.mustChangePassword ? '是' : '否') },
            { key: 'disabled', header: '状态', cell: (row) => (row.disabled ? '已禁用' : '正常') },
            { key: 'created', header: '创建时间', cell: (row) => formatTime(row.createdAt), hideOnTablet: true },
            {
              key: 'actions',
              header: '操作',
              cell: (row) => (
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
              ),
            },
          ]}
        />
      )}
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>新建用户</DialogTitle>
            <DialogDescription>创建后请通过安全渠道传递初始密码。</DialogDescription>
          </DialogHeader>
          <div className="space-y-3">
            <Field label="用户名">
              <Input value={form.username} onChange={(e) => setForm({ ...form, username: e.target.value })} />
            </Field>
            <Field label="初始密码">
              <Input
                type="password"
                value={form.password}
                onChange={(e) => setForm({ ...form, password: e.target.value })}
              />
            </Field>
            <Field label="角色">
              <Select value={form.role} onValueChange={(value) => setForm({ ...form, role: value })}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="viewer">只读</SelectItem>
                  <SelectItem value="operator">操作员</SelectItem>
                  <SelectItem value="admin">管理员</SelectItem>
                </SelectContent>
              </Select>
            </Field>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setOpen(false)}>
              取消
            </Button>
            <Button disabled={!form.username.trim() || form.password.length < 8} onClick={() => create.mutate()}>
              创建用户
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
