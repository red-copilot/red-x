import { zodResolver } from '@hookform/resolvers/zod'
import { KeyRound } from 'lucide-react'
import { useForm } from 'react-hook-form'
import { z } from 'zod'
import { useAuth } from '@/app/auth'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

const schema = z
  .object({
    oldPassword: z.string().min(1, '请输入当前密码'),
    newPassword: z.string().min(8, '新密码至少 8 位'),
    confirmPassword: z.string().min(1, '请再次输入新密码'),
  })
  .refine((value) => value.newPassword === value.confirmPassword, {
    path: ['confirmPassword'],
    message: '两次输入的新密码不一致',
  })
type Values = z.infer<typeof schema>

export function ChangePasswordPage() {
  const { user, changePassword } = useAuth()
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: { oldPassword: '', newPassword: '', confirmPassword: '' },
  })

  async function submit(values: Values) {
    try {
      await changePassword(values.oldPassword, values.newPassword)
      window.location.assign('/login')
    } catch (error) {
      form.setError('root', { message: error instanceof Error ? error.message : '修改密码失败' })
    }
  }

  return (
    <div className="grid min-h-screen place-items-center bg-background p-5">
      <Card className="w-full max-w-md">
        <CardHeader>
          <div className="mb-2 grid size-10 place-items-center rounded-lg bg-secondary text-primary">
            <KeyRound className="size-5" />
          </div>
          <CardTitle className="text-lg">修改登录密码</CardTitle>
          <CardDescription>
            {user?.mustChangePassword
              ? '当前账户要求先修改初始密码，完成后需要重新登录。'
              : '修改后所有现有会话将失效。'}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form className="space-y-4" onSubmit={form.handleSubmit(submit)}>
            <div className="space-y-1.5">
              <Label htmlFor="oldPassword">当前密码</Label>
              <Input
                id="oldPassword"
                type="password"
                autoComplete="current-password"
                {...form.register('oldPassword')}
              />
              {form.formState.errors.oldPassword ? (
                <p className="text-[11px] text-destructive">{form.formState.errors.oldPassword.message}</p>
              ) : null}
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="newPassword">新密码</Label>
              <Input id="newPassword" type="password" autoComplete="new-password" {...form.register('newPassword')} />
              {form.formState.errors.newPassword ? (
                <p className="text-[11px] text-destructive">{form.formState.errors.newPassword.message}</p>
              ) : null}
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="confirmPassword">确认新密码</Label>
              <Input
                id="confirmPassword"
                type="password"
                autoComplete="new-password"
                {...form.register('confirmPassword')}
              />
              {form.formState.errors.confirmPassword ? (
                <p className="text-[11px] text-destructive">{form.formState.errors.confirmPassword.message}</p>
              ) : null}
            </div>
            {form.formState.errors.root ? (
              <div className="rounded-md border border-[#f1b8b2] bg-[#fff8f7] px-3 py-2 text-xs text-[#8f1c13]">
                {form.formState.errors.root.message}
              </div>
            ) : null}
            <Button className="w-full" type="submit" disabled={form.formState.isSubmitting}>
              {form.formState.isSubmitting ? '正在修改…' : '修改密码'}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
