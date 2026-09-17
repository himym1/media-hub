type TauriInvoke = (cmd: string, args?: Record<string, unknown>) => Promise<unknown>

export type CaptureKind = 'hls' | 'file'

export type CaptureItem = {
  url: string
  kind: CaptureKind
}

export type CaptureSnapshot = {
  page: string
  items: CaptureItem[]
}

export type CaptureDownload = {
  kind: CaptureKind
  path: string
  share_import_url?: string | null
}

function tauriInvoke(): TauriInvoke | null {
  const internals = (globalThis as { __TAURI_INTERNALS__?: { invoke?: TauriInvoke } }).__TAURI_INTERNALS__
  return typeof internals?.invoke === 'function' ? internals.invoke.bind(internals) : null
}

export function canCapturePages() {
  return tauriInvoke() !== null
}

export function normalizePageCaptureUrl(url: string) {
  const raw = url.trim()
  if (!raw) return ''
  return raw.includes('://') ? raw : `https://${raw.replace(/^\/\//, '')}`
}

export function isCapturablePageUrl(url: string) {
  const raw = normalizePageCaptureUrl(url)
  if (!raw || raw.length > 8192) return false
  try {
    const parsed = new URL(raw)
    if (parsed.username || parsed.password) return false
    return parsed.protocol === 'http:' || parsed.protocol === 'https:'
  } catch {
    return false
  }
}

export function captureItemLabel(item: CaptureItem) {
  try {
    const parsed = new URL(item.url)
    const name = parsed.pathname.split('/').filter(Boolean).at(-1) || parsed.hostname
    return `${parsed.hostname} · ${decodeURIComponent(name)}`
  } catch {
    return item.kind === 'hls' ? 'HLS 播放列表' : '视频文件'
  }
}

export function captureKindLabel(kind: CaptureKind) {
  return kind === 'file' ? '直链' : 'HLS'
}

export async function openPageCapture(url: string) {
  const invoke = tauriInvoke()
  if (!invoke) throw new Error('只有桌面壳能打开抓取窗口。')
  await invoke('open_page_capture', { url: normalizePageCaptureUrl(url) })
}

export async function listPageCapture() {
  const invoke = tauriInvoke()
  if (!invoke) throw new Error('只有桌面壳能读取抓取结果。')
  const snapshot = await invoke('list_page_capture') as CaptureSnapshot
  return {
    page: snapshot.page ?? '',
    items: Array.isArray(snapshot.items) ? snapshot.items : [],
  } satisfies CaptureSnapshot
}

export async function downloadPageCapture(url: string) {
  const invoke = tauriInvoke()
  if (!invoke) throw new Error('只有桌面壳能下载抓到的地址。')
  return await invoke('download_page_capture', { url }) as CaptureDownload
}

export async function revealPageCapture(path: string) {
  const invoke = tauriInvoke()
  if (!invoke) throw new Error('只有桌面壳能打开下载目录。')
  await invoke('reveal_page_capture', { path })
}
