import {
  Activity,
  Boxes,
  Building2,
  CalendarClock,
  ChevronLeft,
  ChevronRight,
  Command as CommandIcon,
  Crosshair,
  FileKey2,
  FileText,
  Fingerprint,
  GitBranch,
  LayoutDashboard,
  LogOut,
  Menu,
  Radar,
  ScrollText,
  Search,
  Settings2,
  ShieldCheck,
  Users,
  Workflow,
  X,
} from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom'
import { useAuth } from '@/app/auth'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { isAdmin } from '@/lib/access'
import { roleLabel } from '@/lib/format'
import { cn } from '@/lib/utils'
import { CommandPalette } from './command-palette'

type NavItem = { to: string; label: string; icon: typeof Activity; end?: boolean }
type NavGroup = { label: string; items: NavItem[] }

const groups: NavGroup[] = [
  {
    label: '攻击面',
    items: [
      { to: '/', label: '总览', icon: LayoutDashboard, end: true },
      { to: '/organizations', label: '组织', icon: Building2 },
      { to: '/assets', label: '资产', icon: Crosshair },
      { to: '/findings', label: '风险', icon: ShieldCheck },
      { to: '/tasks', label: '任务', icon: Workflow },
    ],
  },
  {
    label: '自动化',
    items: [
      { to: '/automation/monitors', label: '监控', icon: Activity },
      { to: '/automation/schedules', label: '计划任务', icon: CalendarClock },
      { to: '/automation/policies', label: '扫描策略', icon: Settings2 },
      { to: '/automation/groups', label: '资产组', icon: Boxes },
    ],
  },
  {
    label: '情报',
    items: [
      { to: '/intelligence/github', label: 'GitHub', icon: GitBranch },
      { to: '/intelligence/fingerprints', label: '指纹库', icon: Fingerprint },
      { to: '/intelligence/poc', label: 'PoC 插件', icon: Radar },
    ],
  },
]

const adminGroup: NavGroup = {
  label: '管理',
  items: [
    { to: '/admin/users', label: '用户', icon: Users },
    { to: '/admin/api-keys', label: 'API Key', icon: FileKey2 },
    { to: '/admin/audit', label: '审计日志', icon: ScrollText },
    { to: '/admin/system', label: '系统', icon: FileText },
  ],
}

function NavContent({ collapsed, onNavigate }: { collapsed: boolean; onNavigate?: () => void }) {
  const { role } = useAuth()
  const allGroups = isAdmin(role) ? [...groups, adminGroup] : groups
  return (
    <nav className="flex-1 overflow-y-auto px-2 pb-4 scrollbar-thin" aria-label="主导航">
      {allGroups.map((group) => (
        <div key={group.label} className="mb-4">
          {!collapsed ? (
            <div className="px-2 pb-1.5 text-[10px] font-semibold uppercase tracking-[0.12em] text-slate-500">
              {group.label}
            </div>
          ) : null}
          <div className="space-y-0.5">
            {group.items.map((item) => {
              const Icon = item.icon
              const link = (
                <NavLink
                  key={item.to}
                  to={item.to}
                  end={item.end}
                  onClick={onNavigate}
                  className={({ isActive }) =>
                    cn(
                      'flex h-9 items-center gap-2.5 rounded-md px-2.5 text-xs font-medium text-slate-300 transition-colors hover:bg-white/8 hover:text-white',
                      isActive && 'bg-white/10 text-white shadow-[inset_2px_0_0_#7aa7e8]',
                      collapsed && 'justify-center px-0',
                    )
                  }
                >
                  <Icon className="size-4 shrink-0" />
                  {!collapsed ? <span>{item.label}</span> : <span className="sr-only">{item.label}</span>}
                </NavLink>
              )
              return collapsed ? (
                <Tooltip key={item.to}>
                  <TooltipTrigger asChild>{link}</TooltipTrigger>
                  <TooltipContent side="right">{item.label}</TooltipContent>
                </Tooltip>
              ) : (
                link
              )
            })}
          </div>
        </div>
      ))}
    </nav>
  )
}

export function AppShell() {
  const { user, logout } = useAuth()
  const location = useLocation()
  const navigate = useNavigate()
  const [collapsed, setCollapsed] = useState(false)
  const [mobileOpen, setMobileOpen] = useState(false)
  const [searchOpen, setSearchOpen] = useState(false)

  useEffect(() => {
    const handler = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault()
        setSearchOpen(true)
      }
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [])

  useEffect(() => setMobileOpen(false), [location.pathname])

  const title = useMemo(() => {
    const all = [...groups.flatMap((group) => group.items), ...adminGroup.items]
    const exact = all.find((item) => item.to === location.pathname)
    if (exact) return exact.label
    if (location.pathname.startsWith('/organizations/')) return '组织详情'
    if (location.pathname.startsWith('/tasks/')) return '任务详情'
    return 'ARL 控制台'
  }, [location.pathname])

  async function signOut() {
    await logout()
    window.location.assign('/login')
  }

  return (
    <div className="min-h-screen bg-background">
      <aside
        className={cn(
          'fixed inset-y-0 left-0 z-30 hidden flex-col border-r border-slate-800 bg-[#0f1c2e] text-slate-100 transition-[width] lg:flex',
          collapsed ? 'w-[68px]' : 'w-[236px]',
        )}
      >
        <div
          className={cn('flex h-16 items-center border-b border-white/8', collapsed ? 'justify-center' : 'gap-3 px-4')}
        >
          <div className="grid size-9 shrink-0 place-items-center rounded-lg bg-[#dce9fb] text-sm font-black text-[#16365f] shadow-sm">
            A
          </div>
          {!collapsed ? (
            <div className="min-w-0">
              <div className="text-sm font-semibold tracking-wide text-white">ARL</div>
              <div className="truncate text-[10px] uppercase tracking-[0.12em] text-slate-400">
                Asset Reconnaissance
              </div>
            </div>
          ) : null}
        </div>
        <NavContent collapsed={collapsed} />
        <div className="border-t border-white/8 p-2">
          <Button
            variant="ghost"
            size="sm"
            className="w-full justify-center text-slate-400 hover:bg-white/8 hover:text-white"
            onClick={() => setCollapsed((value) => !value)}
          >
            {collapsed ? <ChevronRight /> : <ChevronLeft />}
            {!collapsed ? <span>收起导航</span> : <span className="sr-only">展开导航</span>}
          </Button>
        </div>
      </aside>

      {mobileOpen ? (
        <div className="fixed inset-0 z-50 lg:hidden">
          <button
            className="absolute inset-0 bg-slate-950/35"
            aria-label="关闭导航"
            onClick={() => setMobileOpen(false)}
          />
          <aside className="absolute inset-y-0 left-0 flex w-[280px] flex-col bg-[#0f1c2e] text-white shadow-2xl">
            <div className="flex h-16 items-center justify-between border-b border-white/8 px-4">
              <div className="flex items-center gap-3">
                <div className="grid size-9 place-items-center rounded-lg bg-[#dce9fb] text-sm font-black text-[#16365f]">
                  A
                </div>
                <div>
                  <div className="font-semibold">ARL</div>
                  <div className="text-[10px] uppercase tracking-widest text-slate-400">Asset Reconnaissance</div>
                </div>
              </div>
              <Button
                variant="ghost"
                size="icon"
                className="text-slate-300 hover:bg-white/10 hover:text-white"
                onClick={() => setMobileOpen(false)}
              >
                <X />
                <span className="sr-only">关闭</span>
              </Button>
            </div>
            <NavContent collapsed={false} onNavigate={() => setMobileOpen(false)} />
          </aside>
        </div>
      ) : null}

      <div className={cn('min-h-screen transition-[padding]', collapsed ? 'lg:pl-[68px]' : 'lg:pl-[236px]')}>
        <header className="sticky top-0 z-20 flex h-16 items-center justify-between gap-3 border-b bg-white/95 px-4 backdrop-blur sm:px-6">
          <div className="flex min-w-0 items-center gap-3">
            <Button variant="ghost" size="icon" className="lg:hidden" onClick={() => setMobileOpen(true)}>
              <Menu />
              <span className="sr-only">打开导航</span>
            </Button>
            <div className="min-w-0">
              <div className="truncate text-sm font-semibold">{title}</div>
              <div className="hidden text-[11px] text-muted-foreground sm:block">授权范围内的外部攻击面研判</div>
            </div>
          </div>
          <div className="flex items-center gap-1.5">
            <Button
              variant="outline"
              size="sm"
              className="hidden min-w-52 justify-start text-muted-foreground sm:flex"
              onClick={() => setSearchOpen(true)}
            >
              <Search />
              搜索<span className="ml-auto rounded border px-1.5 py-0.5 text-[10px]">⌘K</span>
            </Button>
            <Button variant="ghost" size="icon" className="sm:hidden" onClick={() => setSearchOpen(true)}>
              <Search />
              <span className="sr-only">搜索</span>
            </Button>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" size="sm" className="max-w-44">
                  <span className="grid size-6 place-items-center rounded-full bg-secondary text-[11px] font-semibold">
                    {user?.username?.slice(0, 1).toUpperCase() || 'A'}
                  </span>
                  <span className="hidden truncate sm:inline">{user?.username || '账户'}</span>
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <div className="px-2.5 py-2 text-xs">
                  <div className="font-medium">{user?.username}</div>
                  <div className="mt-0.5 text-muted-foreground">{roleLabel(user?.role)}</div>
                </div>
                <DropdownMenuSeparator />
                <DropdownMenuItem onSelect={() => navigate('/change-password')}>
                  <CommandIcon />
                  修改密码
                </DropdownMenuItem>
                <DropdownMenuItem onSelect={() => void signOut()}>
                  <LogOut />
                  退出登录
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </header>
        <main className="mx-auto w-full max-w-[1800px] p-4 sm:p-6">
          <Outlet />
        </main>
      </div>
      <CommandPalette open={searchOpen} onOpenChange={setSearchOpen} />
    </div>
  )
}
