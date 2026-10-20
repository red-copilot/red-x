import { Link } from 'react-router-dom'
import { Button } from '@/components/ui/button'

export function NotFoundPage() {
  return (
    <div className="grid min-h-[70vh] place-items-center text-center">
      <div>
        <div className="text-6xl font-black tracking-tighter text-slate-200">404</div>
        <h1 className="mt-3 text-xl font-semibold">页面不存在</h1>
        <p className="mt-2 text-sm text-muted-foreground">旧的 EASM 与 Dashboard 路径已停用，请使用新的攻击面导航。</p>
        <Button asChild className="mt-5">
          <Link to="/">返回总览</Link>
        </Button>
      </div>
    </div>
  )
}
