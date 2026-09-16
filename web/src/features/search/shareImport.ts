const shareHosts = new Set(['115.com', 'www.115.com', '115cdn.com', 'www.115cdn.com', 'anxia.com', 'www.anxia.com'])

export function canSubmitShareImport(url: string, receiveCode = '') {
  try {
    const raw = url.trim()
    if (!raw) return false
    const parsed = new URL(raw.includes('://') ? raw : `https://${raw.replace(/^\/\//, '')}`)
    if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') return false
    if (!shareHosts.has(parsed.hostname.toLowerCase())) return false
    const segments = parsed.pathname.replace(/^\/+|\/+$/g, '').split('/')
    if (segments.length < 2 || segments[0] !== 's' || !/^[A-Za-z0-9]{4,64}$/.test(decodeURIComponent(segments[1] ?? ''))) {
      return false
    }
    const code = (parsed.searchParams.get('password') || parsed.searchParams.get('pwd') || receiveCode).trim()
    return /^[A-Za-z0-9]{0,8}$/.test(code)
  } catch {
    return false
  }
}
