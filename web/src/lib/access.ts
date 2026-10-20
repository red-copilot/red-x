export type Role = 'viewer' | 'operator' | 'admin'

const rank: Record<Role, number> = { viewer: 1, operator: 2, admin: 3 }

export function hasRole(role: Role | undefined, required: Role): boolean {
  return role ? rank[role] >= rank[required] : false
}

export function canWrite(role: Role | undefined): boolean {
  return hasRole(role, 'operator')
}

export function isAdmin(role: Role | undefined): boolean {
  return role === 'admin'
}

export function safeNext(value: string | null | undefined): string {
  if (!value || !value.startsWith('/') || value.startsWith('//')) return '/'
  return value
}
