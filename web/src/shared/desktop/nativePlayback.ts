export type NativeBounds = {
  x: number
  y: number
  width: number
  height: number
}

export type NativePlayRequest = {
  title: string
  startPositionMs?: number
  userAgent?: string
  bounds: NativeBounds
}

export type NativeStatus = {
  paused: boolean
  time: number
  duration: number
  volume: number
  speed: number
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

export function boundsFromElement(element: Element): NativeBounds {
  const rect = element.getBoundingClientRect()
  return { x: rect.left, y: rect.top, width: rect.width, height: rect.height }
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
    bounds: request.bounds,
  })
}

export async function layoutNatively(bounds: NativeBounds) {
  await invokeNative('layout_native', { bounds })
}

export async function stopNatively() {
  await invokeNative('stop_native')
}

export async function controlNatively(action: string, value?: number) {
  await invokeNative('native_control', { action, value })
}

export async function nativeStatus(): Promise<NativeStatus | null> {
  try {
    return await invokeNative('native_status') as NativeStatus
  } catch {
    return null
  }
}
