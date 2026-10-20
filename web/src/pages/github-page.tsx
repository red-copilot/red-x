import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { GitBranch, Plus, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api'
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { formatTime } from '@/lib/format'
import { canWrite } from '@/lib/access'
import { useAuth } from '@/app/auth'

export function GitHubPage() {
  const { role } = useAuth()
  const queryClient = useQueryClient()
  const [tab, setTab] = useState('findings')
  const [open, setOpen] = useState(false)
  const [keyword, setKeyword] = useState('')
  const [name, setName] = useState('')
  const [intervalSec, setIntervalSec] = useState(21600)
  const tasks = useQuery({ queryKey: ['github-tasks'], queryFn: () => api.githubTasks(), enabled: tab === 'tasks' })
  const monitors = useQuery({
    queryKey: ['github-monitors'],
    queryFn: () => api.githubMonitors(),
    enabled: tab === 'monitors',
  })
  const findings = useQuery({
    queryKey: ['github-findings'],
    queryFn: () => api.githubFindings(),
    enabled: tab === 'findings',
  })
  const createTask = useMutation({
    mutationFn: () => api.createGitHubTask(keyword),
    onSuccess: () => {
      toast.success('代码搜索任务已创建')
      setOpen(false)
      setKeyword('')
      void queryClient.invalidateQueries({ queryKey: ['github-tasks'] })
    },
    onError: (error) => toast.error(error.message),
  })
  const createMonitor = useMutation({
    mutationFn: () => api.createGitHubMonitor({ keyword, name, intervalSec }),
    onSuccess: () => {
      toast.success('GitHub 监控已创建')
      setOpen(false)
      void queryClient.invalidateQueries({ queryKey: ['github-monitors'] })
    },
    onError: (error) => toast.error(error.message),
  })
  const remove = useMutation({
    mutationFn: (id: string) => api.deleteGitHubMonitor(id),
    onSuccess: () => {
      toast.success('监控已删除')
      void queryClient.invalidateQueries({ queryKey: ['github-monitors'] })
    },
    onError: (error) => toast.error(error.message),
  })
  return (
    <div>
      <PageHeader
        title="GitHub 情报"
        description="检索公开代码中的域名、凭据泄漏与敏感关键字，并保留去重历史。"
        actions={
          canWrite(role) ? (
            <Button onClick={() => setOpen(true)}>
              <Plus />
              {tab === 'monitors' ? '新建监控' : '新建搜索'}
            </Button>
          ) : null
        }
      />
      <Tabs value={tab} onValueChange={setTab}>
        <TabsList>
          <TabsTrigger value="findings">泄漏结果</TabsTrigger>
          <TabsTrigger value="tasks">搜索任务</TabsTrigger>
          <TabsTrigger value="monitors">周期监控</TabsTrigger>
        </TabsList>
        <TabsContent value="findings">
          {findings.isLoading ? (
            <LoadingBlock />
          ) : findings.error ? (
            <ErrorState error={findings.error} retry={() => findings.refetch()} />
          ) : (
            <DataTable
              rows={findings.data?.items || []}
              rowKey={(row: any) => row.id || row.hashMd5}
              empty={
                <EmptyState
                  title="暂无泄漏结果"
                  action={<GitBranch className="mx-auto mt-2 size-5 text-muted-foreground" />}
                />
              }
              columns={[
                {
                  key: 'repo',
                  header: '仓库',
                  cell: (row: any) =>
                    row.htmlUrl ? (
                      <a className="text-primary hover:underline" href={row.htmlUrl} target="_blank" rel="noreferrer">
                        {row.repo}
                      </a>
                    ) : (
                      row.repo
                    ),
                },
                { key: 'path', header: '文件', cell: (row: any) => <span className="mono">{row.path}</span> },
                { key: 'keyword', header: '关键字', cell: (row: any) => row.keyword },
                { key: 'time', header: '发现时间', cell: (row: any) => formatTime(row.createdAt) },
              ]}
            />
          )}
        </TabsContent>
        <TabsContent value="tasks">
          {tasks.isLoading ? (
            <LoadingBlock />
          ) : (
            <DataTable
              rows={tasks.data?.items || []}
              rowKey={(row: any) => row.id}
              empty={<EmptyState title="暂无 GitHub 搜索任务" />}
              columns={[
                { key: 'keyword', header: '关键字', cell: (row: any) => row.keyword },
                { key: 'status', header: '状态', cell: (row: any) => row.status },
                { key: 'results', header: '结果数', cell: (row: any) => row.resultCount || 0 },
                { key: 'time', header: '创建时间', cell: (row: any) => formatTime(row.createdAt) },
              ]}
            />
          )}
        </TabsContent>
        <TabsContent value="monitors">
          {monitors.isLoading ? (
            <LoadingBlock />
          ) : (
            <DataTable
              rows={monitors.data?.items || []}
              rowKey={(row: any) => row.id}
              empty={<EmptyState title="暂无 GitHub 监控" />}
              columns={[
                { key: 'name', header: '名称', cell: (row: any) => row.name },
                { key: 'keyword', header: '关键字', cell: (row: any) => row.keyword },
                { key: 'status', header: '状态', cell: (row: any) => row.status },
                { key: 'interval', header: '间隔', cell: (row: any) => `${row.intervalSec || 0}s` },
                {
                  key: 'actions',
                  header: '操作',
                  cell: (row: any) =>
                    canWrite(role) ? (
                      <Button
                        size="sm"
                        variant="ghost"
                        className="text-destructive"
                        onClick={() => remove.mutate(row.id)}
                      >
                        <Trash2 />
                        删除
                      </Button>
                    ) : null,
                },
              ]}
            />
          )}
        </TabsContent>
      </Tabs>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{tab === 'monitors' ? '新建 GitHub 监控' : '新建代码搜索'}</DialogTitle>
            <DialogDescription>使用平台配置的 GitHub Token 执行搜索。</DialogDescription>
          </DialogHeader>
          <div className="space-y-3">
            <Field label="关键字">
              <Input value={keyword} onChange={(e) => setKeyword(e.target.value)} placeholder="example.com password" />
            </Field>
            {tab === 'monitors' ? (
              <>
                <Field label="名称">
                  <Input value={name} onChange={(e) => setName(e.target.value)} />
                </Field>
                <Field label="间隔（秒）">
                  <Input type="number" value={intervalSec} onChange={(e) => setIntervalSec(Number(e.target.value))} />
                </Field>
              </>
            ) : null}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setOpen(false)}>
              取消
            </Button>
            <Button
              disabled={!keyword.trim()}
              onClick={() => (tab === 'monitors' ? createMonitor.mutate() : createTask.mutate())}
            >
              创建
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
