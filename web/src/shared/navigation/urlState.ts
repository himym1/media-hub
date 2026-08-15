export type UrlUpdateMode = 'push' | 'replace'

export function updateUrl(url: URL, updates: Record<string, string | null>) {
  const next = new URL(url)
  for (const [key, value] of Object.entries(updates)) {
    if (value) next.searchParams.set(key, value)
    else next.searchParams.delete(key)
  }
  return next
}

export function commitUrl(updates: Record<string, string | null>, mode: UrlUpdateMode = 'push') {
  const next = updateUrl(new URL(window.location.href), updates)
  window.history[mode === 'replace' ? 'replaceState' : 'pushState']({}, '', next)
  return next
}
