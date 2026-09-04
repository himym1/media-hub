export type NativePlayRequest = {
  title: string
  startPositionMs?: number
}

type TauriInvoke = (cmd: string, args: Record<string, unknown>) => Promise<unknown>

function tauriInvoke(): TauriInvoke | null {
  const internals = (globalThis as { __TAURI_INTERNALS__?: { invoke?: TauriInvoke } }).__TAURI_INTERNALS__
  return typeof internals?.invoke === 'function' ? internals.invoke.bind(internals) : null
}

export function canPlayNatively() {
  return tauriInvoke() !== null
}

export async function playNatively(streamUrl: string, request: NativePlayRequest) {
  const invoke = tauriInvoke()
  if (!invoke) throw new Error('native playback is unavailable')
  await invoke('play_native', {
    url: streamUrl,
    title: request.title,
    startPositionMs: request.startPositionMs ?? 0,
  })
}
