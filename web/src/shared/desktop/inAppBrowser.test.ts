import { afterEach, describe, expect, it, vi } from 'vitest'
import { canOpenInApp, openInApp } from './inAppBrowser'

afterEach(() => {
  delete (globalThis as { __TAURI_INTERNALS__?: unknown }).__TAURI_INTERNALS__
})

describe('inAppBrowser', () => {
  it('is off in the browser', () => {
    expect(canOpenInApp()).toBe(false)
  })

  it('opens through Tauri instead of a new tab', async () => {
    const invoke = vi.fn(async () => undefined)
    ;(globalThis as unknown as { __TAURI_INTERNALS__: { invoke: typeof invoke } }).__TAURI_INTERNALS__ = { invoke }
    await openInApp('https://emby.example/web/index.html#!/item?id=1', '在 Emby 打开')
    expect(invoke).toHaveBeenCalledWith('open_in_app', {
      url: 'https://emby.example/web/index.html#!/item?id=1',
      title: '在 Emby 打开',
    })
  })

  it('falls back once when the desktop command is missing', async () => {
    const invoke = vi.fn(async () => {
      throw new Error('missing')
    })
    const open = vi.fn()
    const previous = window.open
    ;(globalThis as unknown as { __TAURI_INTERNALS__: { invoke: typeof invoke } }).__TAURI_INTERNALS__ = { invoke }
    window.open = open as typeof window.open
    try {
      await openInApp('https://emby.example/web/index.html#!/item?id=1')
    } finally {
      window.open = previous
    }
    expect(open).toHaveBeenCalledTimes(1)
    expect(open).toHaveBeenCalledWith(
      'https://emby.example/web/index.html#!/item?id=1',
      '_blank',
      'noopener,noreferrer',
    )
  })
})
