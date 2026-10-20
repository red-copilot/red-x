import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Fingerprint as FingerprintIcon, Plus, Trash2, Upload } from 'lucide-react'
import { useMemo, useState } from 'react'
import { toast } from 'sonner'
import { api, type Fingerprint } from '@/api'
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
import { Textarea } from '@/components/ui/textarea'
import { canWrite } from '@/lib/access'
import { useAuth } from '@/app/auth'

export function FingerprintsPage() {
  const { role } = useAuth()
  const queryClient = useQueryClient()
  const [search, setSearch] = useState('')
  const [open, setOpen] = useState(false)
  const [importOpen, setImportOpen] = useState(false)
  const [form, setForm] = useState({ name: '', rule: '' })
  const [content, setContent] = useState('')
  const fingerprints = useQuery({ queryKey: ['fingerprints'], queryFn: () => api.fingerprints() })
  const refresh = () => queryClient.invalidateQueries({ queryKey: ['fingerprints'] })
  const create = useMutation({
    mutationFn: () => api.createFingerprint(form),
    onSuccess: () => {
      toast.success('指纹已创建')
      setOpen(false)
      setForm({ name: '', rule: '' })
      void refresh()
    },
    onError: (error) => toast.error(error.message),
  })
  const remove = useMutation({
    mutationFn: (id: string) => api.deleteFingerprint(id),
    onSuccess: () => {
      toast.success('指纹已删除')
      void refresh()
    },
    onError: (error) => toast.error(error.message),
  })
  const importRules = useMutation({
    mutationFn: () => api.importFingerprints(content),
    onSuccess: () => {
      toast.success('指纹导入完成')
      setImportOpen(false)
      setContent('')
      void refresh()
    },
    onError: (error) => toast.error(error.message),
  })
  const rows = useMemo(
    () =>
      ((fingerprints.data?.items || []) as Fingerprint[]).filter(
        (row) => !search || `${row.name} ${row.rule}`.toLowerCase().includes(search.toLowerCase()),
      ),
    [fingerprints.data, search],
  )
  return (
    <div>
      <PageHeader
        title="指纹库"
        description="布尔表达式规则覆盖 body、header、title 与 icon_hash，用于站点识别。"
        actions={
          canWrite(role) ? (
            <>
              <Button variant="outline" onClick={() => setImportOpen(true)}>
                <Upload />
                批量导入
              </Button>
              <Button onClick={() => setOpen(true)}>
                <Plus />
                新增指纹
              </Button>
            </>
          ) : null
        }
      />
      <div className="mb-3 max-w-sm">
        <Input value={search} onChange={(e) => setSearch(e.target.value)} placeholder="搜索指纹名称或规则…" />
      </div>
      {fingerprints.isLoading ? (
        <LoadingBlock />
      ) : fingerprints.error ? (
        <ErrorState error={fingerprints.error} retry={() => fingerprints.refetch()} />
      ) : (
        <DataTable
          rows={rows}
          rowKey={(row) => row.id || row.name || ''}
          empty={
            <EmptyState
              title="暂无指纹"
              action={<FingerprintIcon className="mx-auto mt-2 size-5 text-muted-foreground" />}
            />
          }
          columns={[
            { key: 'name', header: '名称', cell: (row) => row.name },
            {
              key: 'rule',
              header: '规则',
              cell: (row) => <span className="mono max-w-[620px] truncate text-[11px]">{row.rule}</span>,
            },
            {
              key: 'actions',
              header: '操作',
              cell: (row) =>
                canWrite(role) ? (
                  <Button
                    size="sm"
                    variant="ghost"
                    className="text-destructive"
                    onClick={() => remove.mutate(row.id || '')}
                  >
                    <Trash2 />
                    删除
                  </Button>
                ) : null,
            },
          ]}
        />
      )}
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>新增指纹</DialogTitle>
            <DialogDescription>使用 WIH 布尔表达式。</DialogDescription>
          </DialogHeader>
          <div className="space-y-3">
            <Field label="名称">
              <Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
            </Field>
            <Field label="规则">
              <Textarea
                value={form.rule}
                onChange={(e) => setForm({ ...form, rule: e.target.value })}
                placeholder='body="example" || title="Example"'
              />
            </Field>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setOpen(false)}>
              取消
            </Button>
            <Button disabled={!form.name.trim() || !form.rule.trim()} onClick={() => create.mutate()}>
              创建指纹
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      <Dialog open={importOpen} onOpenChange={setImportOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>批量导入指纹</DialogTitle>
            <DialogDescription>支持 YAML 或 JSON 规则数组。</DialogDescription>
          </DialogHeader>
          <Textarea
            value={content}
            onChange={(e) => setContent(e.target.value)}
            className="min-h-64 font-mono text-xs"
            placeholder={'- name: Example\n  rule: body="example"'}
          />
          <DialogFooter>
            <Button variant="outline" onClick={() => setImportOpen(false)}>
              取消
            </Button>
            <Button disabled={!content.trim()} onClick={() => importRules.mutate()}>
              导入
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
