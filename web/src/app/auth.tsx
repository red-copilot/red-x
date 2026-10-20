import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { Navigate, useLocation } from 'react-router-dom'
import { api, type User } from '@/api'
import { isAdmin, type Role } from '@/lib/access'

type AuthContextValue = {
  user: User | null
  role?: Role
  loading: boolean
  login: (username: string, password: string) => Promise<User>
  logout: () => Promise<void>
  changePassword: (oldPassword: string, newPassword: string) => Promise<void>
  refresh: () => Promise<void>
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)

  const refresh = useCallback(async () => {
    try {
      const principal = await api.me()
      setUser(
        (principal.user as User | null) || {
          username: principal.username,
          role: principal.role as Role,
        },
      )
    } catch {
      setUser(null)
    }
  }, [])

  useEffect(() => {
    refresh().finally(() => setLoading(false))
    const unauthorized = () => setUser(null)
    window.addEventListener('arl:unauthorized', unauthorized)
    return () => window.removeEventListener('arl:unauthorized', unauthorized)
  }, [refresh])

  const login = useCallback(async (username: string, password: string) => {
    const result = await api.login(username, password)
    const nextUser = result.user as User
    setUser(nextUser)
    return nextUser
  }, [])

  const logout = useCallback(async () => {
    try {
      await api.logout()
    } finally {
      setUser(null)
    }
  }, [])

  const changePassword = useCallback(async (oldPassword: string, newPassword: string) => {
    await api.changePassword(oldPassword, newPassword)
    setUser(null)
  }, [])

  const value = useMemo<AuthContextValue>(
    () => ({ user, role: user?.role as Role | undefined, loading, login, logout, changePassword, refresh }),
    [user, loading, login, logout, changePassword, refresh],
  )
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const context = useContext(AuthContext)
  if (!context) throw new Error('useAuth must be used inside AuthProvider')
  return context
}

export function ProtectedRoute({ children }: { children: ReactNode }) {
  const { user, loading } = useAuth()
  const location = useLocation()
  if (loading)
    return <div className="grid min-h-screen place-items-center text-sm text-muted-foreground">正在验证会话…</div>
  if (!user) return <Navigate to={`/login?next=${encodeURIComponent(location.pathname + location.search)}`} replace />
  if (user.mustChangePassword && location.pathname !== '/change-password') {
    return <Navigate to="/change-password" replace />
  }
  return <>{children}</>
}

export function AdminRoute({ children }: { children: ReactNode }) {
  const { role } = useAuth()
  return isAdmin(role) ? <>{children}</> : <Navigate to="/" replace />
}
