import { canWrite, hasRole, safeNext } from './access'

describe('access helpers', () => {
  it('allows admins to perform write and admin actions', () => {
    expect(canWrite('admin')).toBe(true)
    expect(hasRole('admin', 'admin')).toBe(true)
    expect(hasRole('admin', 'operator')).toBe(true)
  })

  it('allows operators to write but not administer', () => {
    expect(canWrite('operator')).toBe(true)
    expect(hasRole('operator', 'operator')).toBe(true)
    expect(hasRole('operator', 'admin')).toBe(false)
  })

  it('keeps viewers read-only', () => {
    expect(canWrite('viewer')).toBe(false)
    expect(hasRole('viewer', 'operator')).toBe(false)
  })

  it('only accepts local return paths', () => {
    expect(safeNext('/organizations/o1?tab=domains')).toBe('/organizations/o1?tab=domains')
    expect(safeNext('//evil.example')).toBe('/')
    expect(safeNext('https://evil.example')).toBe('/')
  })
})
