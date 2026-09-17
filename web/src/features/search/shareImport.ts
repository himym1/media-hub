const shareHosts = new Set(['115.com', 'www.115.com', '115cdn.com', 'www.115cdn.com', 'anxia.com', 'www.anxia.com'])
const otherCloudHosts = new Set([
  'pan.quark.cn',
  'www.pan.quark.cn',
  'aliyundrive.com',
  'www.aliyundrive.com',
  'alipan.com',
  'www.alipan.com',
  'pan.baidu.com',
  'www.pan.baidu.com',
  'yun.baidu.com',
  '123pan.com',
  'www.123pan.com',
])
const videoExt = /\.(mp4|mkv|avi|ts|m2ts|mts|wmv|flv|mov|iso|rmvb|rm|m4v|webm|mpeg|mpg|vob|f4v|asf|3gp)$/i

export function canSubmitShareImport(url: string, receiveCode = '') {
  const raw = url.trim()
  if (!raw || raw.length > 8192) return false
  const lower = raw.toLowerCase()
  if (lower.startsWith('magnet:')) return lower.includes('xt=urn:btih:')
  if (lower.startsWith('ed2k://')) return raw.length >= 16
  try {
    const parsed = new URL(raw.includes('://') ? raw : `https://${raw.replace(/^\/\//, '')}`)
    if (parsed.username || parsed.password) return false
    if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') return false
    const host = parsed.hostname.toLowerCase()
    if (otherCloudHosts.has(host)) return false
    if (shareHosts.has(host)) {
      const segments = parsed.pathname.replace(/^\/+|\/+$/g, '').split('/')
      if (segments.length >= 2 && segments[0] === 's' && /^[A-Za-z0-9]{4,64}$/.test(decodeURIComponent(segments[1] ?? ''))) {
        const code = (parsed.searchParams.get('password') || parsed.searchParams.get('pwd') || receiveCode).trim()
        return /^[A-Za-z0-9]{0,8}$/.test(code)
      }
      return videoExt.test(parsed.pathname)
    }
    return true
  } catch {
    return false
  }
}
