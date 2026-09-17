import type { EmbyPerson } from '../../shared/api/mediaHub'

export function formatPersonRole(person: EmbyPerson): string {
  const type = (person.type ?? '').trim().toLowerCase()
  const role = person.role?.trim()
  if (type === 'director') return '导演'
  if (type === 'writer') return '编剧'
  if (type === 'producer') return '制片人'
  if (type === 'creator') return '主创'
  if (role) {
    if (role.toLowerCase() === 'director') return '导演'
    if (role.toLowerCase() === 'writer') return '编剧'
    return `饰 ${role}`
  }
  if (type === 'actor') return '演员'
  if (type === 'gueststar') return '客串'
  return person.type || '演职员'
}

export function personInitials(name: string): string {
  const trimmed = name.trim()
  if (!trimmed) return '?'
  if (/^[\u4e00-\u9fa5]/.test(trimmed)) {
    return trimmed.slice(0, 2)
  }
  const parts = trimmed.split(/\s+/)
  if (parts.length >= 2) {
    return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase()
  }
  return trimmed.slice(0, 2).toUpperCase()
}
