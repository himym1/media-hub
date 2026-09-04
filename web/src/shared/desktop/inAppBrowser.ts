type TauriInvoke = (cmd: string, args?: Record<string, unknown>) => Promise<unknown>
type WindowOpen = typeof window.open

let systemOpen: WindowOpen | null = null

function tauriInvoke(): TauriInvoke | null {
  const internals = (globalThis as { __TAURI_INTERNALS__?: { invoke?: TauriInvoke } }).__TAURI_INTERNALS__
  return typeof internals?.invoke === 'function' ? internals.invoke.bind(internals) : null
}

function openInSystemBrowser(url: string) {
  const opener = systemOpen ?? (typeof window === 'undefined' ? undefined : window.open.bind(window))
  opener?.(url, '_blank', 'noopener,noreferrer')
}

export function canOpenInApp() {
  return tauriInvoke() !== null
}

export async function openInApp(url: string, title?: string) {
  const invoke = tauriInvoke()
  if (invoke) {
    try {
      await invoke('open_in_app', { url, title: title ?? '' })
      return
    } catch {
      // Older desktop builds fall through to the system browser.
    }
  }
  openInSystemBrowser(url)
}

export function bindInAppLinks() {
  if (!canOpenInApp() || typeof document === 'undefined') return () => undefined

  const onClick = (event: MouseEvent) => {
    if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) {
      return
    }
    const target = event.target
    if (!(target instanceof Element)) return
    const link = target.closest('a')
    if (!link || link.target !== '_blank') return
    const href = link.href
    if (!href.startsWith('http://') && !href.startsWith('https://')) return
    event.preventDefault()
    void openInApp(href, link.textContent?.trim())
  }

  const originalOpen = window.open.bind(window)
  systemOpen = originalOpen
  window.open = ((url?: string | URL, target?: string, features?: string) => {
    const href = typeof url === 'string' ? url : url?.toString()
    if (href && (href.startsWith('http://') || href.startsWith('https://'))) {
      void openInApp(href)
      return null
    }
    return originalOpen(url, target, features)
  }) as WindowOpen

  document.addEventListener('click', onClick)
  return () => {
    document.removeEventListener('click', onClick)
    window.open = originalOpen
    if (systemOpen === originalOpen) {
      systemOpen = null
    }
  }
}
