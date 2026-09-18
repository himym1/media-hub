export const DESKTOP_NAVIGATE_EVENT = 'media-hub-navigate'
export const DESKTOP_LOGOUT_EVENT = 'media-hub-logout'

type TauriInternals = { invoke?: (...args: never[]) => unknown }

export function isDesktopShell() {
  const internals = (globalThis as { __TAURI_INTERNALS__?: TauriInternals }).__TAURI_INTERNALS__
  return typeof internals?.invoke === 'function'
}

export function desktopPlatform(): 'macos' | 'windows' | 'linux' | 'web' {
  if (!isDesktopShell() || typeof navigator === 'undefined') return 'web'
  if (/Mac OS X|Macintosh/.test(navigator.userAgent)) return 'macos'
  if (/Windows/.test(navigator.userAgent)) return 'windows'
  return 'linux'
}

export function markDesktopShell() {
  if (typeof document === 'undefined' || !isDesktopShell()) return
  const root = document.documentElement
  root.dataset.appShell = 'desktop'
  root.dataset.platform = desktopPlatform()
}

export function workspaceViewFromDesktopEvent(event: Event) {
  const view = (event as CustomEvent<{ view?: unknown }>).detail?.view
  return typeof view === 'string' && view.length > 0 ? view : null
}
