import { ApiError, completeCaptureUpload, type CaptureUploadTicket } from '../api/mediaHub'

type TauriInvoke = (cmd: string, args?: Record<string, unknown>) => Promise<unknown>

export type CaptureKind = 'hls' | 'file'

export type CaptureItem = {
  url: string
  kind: CaptureKind
}

export type CaptureDownload = {
  kind: CaptureKind
  path: string
  size?: number
  title?: string
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

export function pickAutoCaptureItem(items: CaptureItem[]) {
  return [...items].reverse().find((item) => item.kind === 'hls')
    ?? [...items].reverse().find((item) => item.kind === 'file')
}

function isMissingCommand(cause: unknown) {
  const message = cause instanceof Error ? cause.message : String(cause)
  return /run_page_capture/i.test(message) && /not found|unknown/i.test(message)
}

async function runLegacyPageCapture(invoke: TauriInvoke, url: string) {
  await invoke('open_page_capture', { url })
  const deadline = Date.now() + 45_000
  let items: CaptureItem[] = []
  while (Date.now() < deadline) {
    const snapshot = await invoke('list_page_capture') as { items?: CaptureItem[] }
    items = Array.isArray(snapshot.items) ? snapshot.items : []
    if (pickAutoCaptureItem(items)) break
    await new Promise((resolve) => setTimeout(resolve, 400))
  }
  const item = pickAutoCaptureItem(items)
  if (!item) throw new Error('页面里没有明文 m3u8/mp4。可能是登录墙、同意页，或分片加密。')
  return await invoke('download_page_capture', { url: item.url }) as CaptureDownload
}

export async function runPageCapture(url: string) {
  const invoke = tauriInvoke()
  if (!invoke) throw new Error('只有桌面壳能抓取网页。')
  const normalized = normalizePageCaptureUrl(url)
  try {
    return await invoke('run_page_capture', { url: normalized }) as CaptureDownload
  } catch (cause) {
    if (!isMissingCommand(cause)) throw cause
    return await runLegacyPageCapture(invoke, normalized)
  }
}

export async function revealPageCapture(path: string) {
  const invoke = tauriInvoke()
  if (!invoke) throw new Error('只有桌面壳能打开下载目录。')
  await invoke('reveal_page_capture', { path })
}

export function captureFileName(path: string) {
  const parts = path.split(/[/\\]/).filter(Boolean)
  return parts.at(-1) ?? ''
}

export async function uploadPageCapture(path: string, ticket: CaptureUploadTicket) {
  const invoke = tauriInvoke()
  if (!invoke) throw new Error('只有桌面壳能上传抓取文件。')
  await invoke('upload_page_capture', { path, ticket })
}

export async function completeCaptureUploadWhenReady(
  input: { destinationId: string; filename: string; title?: string },
  idempotencyKey: string,
) {
  const deadline = Date.now() + 45_000
  let last: unknown
  while (Date.now() < deadline) {
    try {
      return await completeCaptureUpload(input, idempotencyKey)
    } catch (cause) {
      last = cause
      if (!(cause instanceof ApiError) || cause.code !== 'capture_upload_not_ready') throw cause
      await new Promise((resolve) => setTimeout(resolve, 1500))
    }
  }
  throw last instanceof Error ? last : new Error('115 还没有收到抓取文件')
}
