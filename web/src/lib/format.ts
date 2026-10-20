import type { Role } from './access'

export function formatTime(value?: string | null): string {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString('zh-CN', { hour12: false })
}

export function formatNumber(value?: number | null): string {
  return new Intl.NumberFormat('zh-CN').format(value ?? 0)
}

export function relativeTime(value?: string | null): string {
  if (!value) return '—'
  const delta = Date.now() - new Date(value).getTime()
  if (Number.isNaN(delta)) return value
  const minutes = Math.max(0, Math.floor(delta / 60000))
  if (minutes < 1) return '刚刚'
  if (minutes < 60) return `${minutes} 分钟前`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours} 小时前`
  return `${Math.floor(hours / 24)} 天前`
}

export function riskLabel(risk?: string): string {
  return (
    ({ critical: '严重', high: '高危', medium: '中危', low: '低危', info: '信息' } as Record<string, string>)[
      risk || ''
    ] ||
    risk ||
    '未知'
  )
}

export function statusLabel(status?: string): string {
  const labels: Record<string, string> = {
    queued: '排队中',
    running: '运行中',
    succeeded: '已完成',
    partial: '部分完成',
    failed: '失败',
    canceled: '已取消',
    active: '启用',
    paused: '暂停',
    candidate: '待确认',
    confirmed: '已确认',
    rejected: '已拒绝',
    done: '完成',
  }
  return labels[status || ''] || status || '—'
}

export function roleLabel(role?: Role): string {
  return ({ viewer: '只读', operator: '操作员', admin: '管理员' } as Record<string, string>)[role || ''] || role || '—'
}

export function policySummary(config?: Record<string, any>): string {
  if (!config) return '—'
  const groups = ['domain', 'ip', 'site'].flatMap((key) => Object.entries(config[key] || {}))
  return (
    groups
      .filter(([, value]) => value === true)
      .map(([key]) => key)
      .join(' · ') || '无启用项'
  )
}
