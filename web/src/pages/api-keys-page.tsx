import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Copy, FileKey2, Plus, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { api, type APIKey } from '@/api'
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
import { Textarea } from '@/components/ui/textarea'
import { formatTime } from '@/lib/format'

export function APIKeysPage() {
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)
  const [name, setName] = useState('')
  const [scopes, setScopes] = useState('read')
  const [secret, setSecret] = useState('')
  const keys = useQuery({ queryKey: ['api-keys'], queryFn: () => api.apiKeys() })
  const refresh = () => queryClient.invalidateQueries({ queryKey: ['api-keys'] })
  const create = useMutation({
    mutationFn: () =>
      api.createAPIKey({
        name,
        scopes: scopes
          .split(',')
          .map((item) => item.trim())
          .filter(Boolean),
      }),
    onSuccess: (result) => {
      setSecret(result.secret || '')
      setOpen(false)
      setName('')
      void refresh()
    },
    onError: (error) => toast.error(error.message),
  })
  const remove = useMutation({
    mutationFn: (id: string) => api.deleteAPIKey(id),
    onSuccess: () => {
      toast.success('API Key 已吊销')
      void refresh()
    },
    onError: (error) => toast.error(error.message),
  })
  const rows = (keys.data?.items || []) as APIKey[]
  return (
    <div>
      <PageHeader
        title="API Key"
        description="为自动化脚本和外部集成创建 API Key。密钥仅在创建时显示一次。"
        actions={
          <Button onClick={() => setOpen(true)}>
            <Plus />
            新建 Key
          </Button>
        }
      />
      {keys.isLoading ? (
        <LoadingBlock />
      ) : keys.error ? (
        <ErrorState error={keys.error} retry={() => keys.refetch()} />
      ) : (
        <DataTable
          rows={rows}
          rowKey={(row) => row.id || row.name || ''}
          empty={
            <EmptyState
              title="暂无 API Key"
              action={<FileKey2 className="mx-auto mt-2 size-5 text-muted-foreground" />}
            />
          }
          columns={[
            { key: 'name', header: '名称', cell: (row) => <span className="font-medium">{row.name}</span> },
            {
              key: 'scopes',
              header: '权限',
              cell: (row) => (
                <div className="flex flex-wrap gap-1">
                  {row.scopes?.map((scope) => (
                    <Badge key={scope} variant="outline">
                      {scope}
                    </Badge>
                  ))}
                </div>
              ),
            },
            { key: 'created', header: '创建时间', cell: (row) => formatTime(row.createdAt), hideOnTablet: true },
            { key: 'lastUsed', header: '最近使用', cell: (row) => formatTime(row.lastUsedAt), hideOnTablet: true },
            {
              key: 'status',
              header: '状态',
              cell: (row) =>
                row.revokedAt ? <Badge variant="danger">已吊销</Badge> : <Badge variant="success">有效</Badge>,
            },
            {
              key: 'actions',
              header: '操作',
              cell: (row) =>
                !row.revokedAt ? (
                  <div className="flex justify-end">
                    <Button
                      size="sm"
                      variant="ghost"
                      className="text-destructive"
                      onClick={() => remove.mutate(row.id || '')}
                    >
                      <Trash2 />
                      吊销
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
            <DialogTitle>新建 API Key</DialogTitle>
            <DialogDescription>权限使用逗号分隔，例如 read,scan。</DialogDescription>
          </DialogHeader>
          <div className="space-y-3">
            <Field label="名称">
              <Input value={name} onChange={(e) => setName(e.target.value)} />
            </Field>
            <Field label="权限">
              <Input value={scopes} onChange={(e) => setScopes(e.target.value)} />
            </Field>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setOpen(false)}>
              取消
            </Button>
            <Button disabled={!name.trim()} onClick={() => create.mutate()}>
              创建 Key
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      <Dialog open={Boolean(secret)} onOpenChange={(next) => !next && setSecret('')}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>保存 API Key</DialogTitle>
            <DialogDescription>该密钥只显示一次，关闭后无法再次读取。</DialogDescription>
          </DialogHeader>
          <Textarea readOnly value={secret} className="min-h-28 font-mono text-xs" />
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => {
                void navigator.clipboard.writeText(secret)
                toast.success('已复制')
              }}
            >
              <Copy />
              复制
            </Button>
            <Button onClick={() => setSecret('')}>我已保存</Button>
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
