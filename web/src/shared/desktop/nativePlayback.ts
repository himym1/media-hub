export type NativePlayRequest = {
  title: string
  startPositionMs?: number
  userAgent?: string
}

type TauriInvoke = (cmd: string, args: Record<string, unknown>) => Promise<unknown>

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

export async function playNatively(streamUrl: string, request: NativePlayRequest) {
  const invoke = tauriInvoke()
  if (!invoke) throw new Error('native playback is unavailable')
  try {
    await invoke('play_native', {
      url: streamUrl,
      title: request.title,
      startPositionMs: request.startPositionMs ?? 0,
      userAgent: request.userAgent ?? '',
    })
  } catch (cause) {
    throw new Error(nativePlayErrorMessage(cause))
  }
}
