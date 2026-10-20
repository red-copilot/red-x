import { zodResolver } from '@hookform/resolvers/zod'
import { ArrowRight, LockKeyhole, ShieldCheck } from 'lucide-react'
import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { z } from 'zod'
import { useAuth } from '@/app/auth'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { safeNext } from '@/lib/access'

const schema = z.object({
  username: z.string().trim().min(1, '请输入用户名'),
  password: z.string().min(1, '请输入密码'),
})
type FormValues = z.infer<typeof schema>

export function LoginPage() {
  const { user, login } = useAuth()
  const [params] = useSearchParams()
  const navigate = useNavigate()
  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { username: 'admin', password: '' },
  })

  useEffect(() => {
    if (user) navigate(safeNext(params.get('next')), { replace: true })
  }, [user, navigate, params])

  async function submit(values: FormValues) {
    try {
      const nextUser = await login(values.username, values.password)
      navigate(nextUser.mustChangePassword ? '/change-password' : safeNext(params.get('next')), { replace: true })
    } catch (error) {
      form.setError('root', { message: error instanceof Error ? error.message : '登录失败' })
    }
  }

  return (
    <div className="grid min-h-screen bg-[#f4f6f9] lg:grid-cols-[1.05fr_0.95fr]">
      <section className="relative hidden overflow-hidden bg-[#10233c] p-12 text-white lg:flex lg:flex-col lg:justify-between">
        <div className="absolute inset-0 opacity-30 data-grid" />
        <div className="relative flex items-center gap-3">
          <div className="grid size-11 place-items-center rounded-xl bg-[#e3edfb] text-lg font-black text-[#16365f]">
            A
          </div>
          <div>
            <div className="text-lg font-semibold tracking-wide">ARL</div>
            <div className="text-[10px] uppercase tracking-[0.18em] text-slate-400">
              Asset Reconnaissance Lighthouse
            </div>
          </div>
        </div>
        <div className="relative max-w-xl">
          <div className="mb-5 inline-flex items-center gap-2 rounded-full border border-white/15 bg-white/5 px-3 py-1 text-[11px] text-slate-300">
            <ShieldCheck className="size-3.5" />
            仅用于授权安全测试
          </div>
          <h1 className="text-4xl font-semibold leading-tight tracking-tight">从组织出发，看清每一层外部暴露面。</h1>
          <p className="mt-5 max-w-lg text-sm leading-7 text-slate-300">
            把单位、备案主域、资产、风险与扫描任务放在同一条研判路径中。地图与关系图共享选择，证据与操作保持可追溯。
          </p>
        </div>
        <div className="relative grid grid-cols-3 gap-4 border-t border-white/10 pt-5 text-[11px] text-slate-400">
          <span>
            <b className="mb-1 block text-base text-white">组织</b>归属与域名治理
          </span>
          <span>
            <b className="mb-1 block text-base text-white">资产</b>DNS / IP / 服务
          </span>
          <span>
            <b className="mb-1 block text-base text-white">风险</b>当前发现与历史
          </span>
        </div>
      </section>
      <section className="flex items-center justify-center px-5 py-10 sm:px-10">
        <div className="w-full max-w-md">
          <div className="mb-8 lg:hidden">
            <div className="flex items-center gap-3">
              <div className="grid size-10 place-items-center rounded-lg bg-[#16365f] font-black text-white">A</div>
              <div>
                <div className="font-semibold">ARL</div>
                <div className="text-[10px] uppercase tracking-widest text-muted-foreground">Asset Reconnaissance</div>
              </div>
            </div>
          </div>
          <div className="mb-7">
            <div className="mb-2 flex items-center gap-2 text-xs font-medium text-primary">
              <LockKeyhole className="size-4" />
              控制台登录
            </div>
            <h2 className="text-2xl font-semibold tracking-tight">进入攻击面工作区</h2>
            <p className="mt-2 text-sm text-muted-foreground">使用平台账户登录。首次登录后需要修改初始密码。</p>
          </div>
          <form className="space-y-4" onSubmit={form.handleSubmit(submit)}>
            <div className="space-y-1.5">
              <Label htmlFor="username">用户名</Label>
              <Input id="username" autoComplete="username" {...form.register('username')} />
              {form.formState.errors.username ? (
                <p className="text-[11px] text-destructive">{form.formState.errors.username.message}</p>
              ) : null}
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="password">密码</Label>
              <Input id="password" type="password" autoComplete="current-password" {...form.register('password')} />
              {form.formState.errors.password ? (
                <p className="text-[11px] text-destructive">{form.formState.errors.password.message}</p>
              ) : null}
            </div>
            {form.formState.errors.root ? (
              <div className="rounded-md border border-[#f1b8b2] bg-[#fff8f7] px-3 py-2 text-xs text-[#8f1c13]">
                {form.formState.errors.root.message}
              </div>
            ) : null}
            <Button className="w-full" size="lg" type="submit" disabled={form.formState.isSubmitting}>
              {form.formState.isSubmitting ? '正在登录…' : '登录'}
              <ArrowRight />
            </Button>
          </form>
          <p className="mt-6 text-center text-[11px] leading-5 text-muted-foreground">
            首次启动的管理员密码仅在服务日志中输出一次。请勿在非授权目标上使用本系统。
          </p>
        </div>
      </section>
    </div>
  )
}
