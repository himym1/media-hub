export type NativeBounds = {
  x: number
  y: number
  width: number
  height: number
}

export type NativeSubtitle = {
  base64: string
  fileName: string
}

export type NativePlayRequest = {
  title: string
  startPositionMs?: number
  userAgent?: string
  subtitle?: NativeSubtitle | null
}

export type NativeStatus = {
  paused: boolean
  time: number
  duration: number
  volume: number
  speed: number
  zoom?: number
  cursorHover?: boolean
  mouseX?: number
  mouseY?: number
  fullscreen?: boolean
  running?: boolean
  subtitles?: boolean
}

type TauriInvoke = (cmd: string, args?: Record<string, unknown>) => Promise<unknown>

function tauriInvoke(): TauriInvoke | null {
  const internals = (globalThis as { __TAURI_INTERNALS__?: { invoke?: TauriInvoke } }).__TAURI_INTERNALS__
  return typeof internals?.invoke === 'function' ? internals.invoke.bind(internals) : null
}

export function canPlayNatively() {
  return tauriInvoke() !== null
}

export function nativePlayErrorMessage(cause: unknown) {
  const raw = typeof cause === 'string'
    ? cause
    : cause instanceof Error
      ? cause.message
      : ''
  if (!raw || /https?:\/\//i.test(raw)) {
    return '系统播放器未能打开这路流。'
  }
  return raw
}

/// 0.21.29 之前的桌面壳按这块矩形把 mpv 嵌进网页，新壳忽略它。
function legacyEmbedBounds(): NativeBounds {
  return { x: 0, y: 0, width: window.innerWidth, height: window.innerHeight }
}

export function bytesToBase64(buffer: ArrayBuffer) {
  const bytes = new Uint8Array(buffer)
  const chunk = 0x8000
  let binary = ''
  for (let offset = 0; offset < bytes.length; offset += chunk) {
    binary += String.fromCharCode(...bytes.subarray(offset, offset + chunk))
  }
  return btoa(binary)
}

export function nativeSubtitleFromBytes(bytes: ArrayBuffer, fileName: string): NativeSubtitle | null {
  if (bytes.byteLength === 0) return null
  return {
    base64: bytesToBase64(bytes),
    fileName: fileName.trim() || 'chi.srt',
  }
}

async function invokeNative(cmd: string, args: Record<string, unknown> = {}) {
  const invoke = tauriInvoke()
  if (!invoke) throw new Error('native playback is unavailable')
  try {
    return await invoke(cmd, args)
  } catch (cause) {
    throw new Error(nativePlayErrorMessage(cause))
  }
}

export async function playNatively(streamUrl: string, request: NativePlayRequest) {
  await invokeNative('play_native', {
    url: streamUrl,
    title: request.title,
    startPositionMs: request.startPositionMs ?? 0,
    userAgent: request.userAgent ?? '',
    bounds: legacyEmbedBounds(),
    subtitleBase64: request.subtitle?.base64 ?? '',
    subtitleFileName: request.subtitle?.fileName ?? '',
  })
}

export async function attachNativeSubtitle(subtitle: NativeSubtitle) {
  await invokeNative('attach_native_subtitle', {
    subtitleBase64: subtitle.base64,
    subtitleFileName: subtitle.fileName,
  })
}

export async function stopNatively() {
  await invokeNative('stop_native')
}

export async function controlNatively(action: string, value?: number, mode?: string) {
  await invokeNative('native_control', { action, value, mode })
}

export async function nativeStatus(): Promise<NativeStatus | null> {
  try {
    return await invokeNative('native_status') as NativeStatus
  } catch {
    return null
  }
}

export type OpenPlayerWindowRequest = {
  playId: string
  title?: string
  seriesId?: string
}

export async function openPlayerWindow(request: OpenPlayerWindowRequest) {
  await invokeNative('open_player_window', {
    playId: request.playId,
    title: request.title ?? '',
    seriesId: request.seriesId ?? '',
  })
}

export async function closePlayerWindow() {
  try {
    await invokeNative('close_player_window')
  } catch {
    if (typeof window.close === 'function') window.close()
  }
}
