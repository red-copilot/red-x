import { defineStore } from 'pinia'
import { api } from '@/api'

type User = {
  id?: string
  username: string
  role: 'viewer' | 'operator' | 'admin'
  mustChangePassword?: boolean
}

let ready: Promise<void> | null = null

export const useAuthStore = defineStore('auth', {
  state: () => ({
    user: null as User | null,
    loading: false,
  }),
  getters: {
    isAdmin: (state) => state.user?.role === 'admin',
    canWrite: (state) => state.user?.role === 'admin' || state.user?.role === 'operator',
  },
  actions: {
    async login(username: string, password: string) {
      const res = await api<{ data: { user: User } }>('/auth/login', {
        method: 'POST',
        body: JSON.stringify({ username, password }),
      })
      this.user = res.data.user
    },
    async fetchMe(force = false) {
      if (!force && this.user) return
      if (ready) return ready
      ready = (async () => {
        try {
          const res = await api<{ data: { user: User | null; username: string; role: User['role'] } }>('/auth/me')
          this.user = res.data.user || { username: res.data.username, role: res.data.role }
        } catch {
          this.user = null
        } finally {
          ready = null
        }
      })()
      return ready
    },
    async logout() {
      try {
        await api('/auth/logout', { method: 'POST', body: JSON.stringify({}) })
      } finally {
        this.user = null
      }
    },
  },
})
