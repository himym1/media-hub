import { afterEach, describe, expect, it } from 'vitest'
import {
  desktopPlatform,
  isApplePlatform,
  isDesktopShell,
  markDesktopShell,
  primaryShortcutModifier,
  workspaceViewFromDesktopEvent,
} from './desktopShell'

const root = { dataset: {} as Record<string, string | undefined> }

afterEach(() => {
  delete (globalThis as { __TAURI_INTERNALS__?: unknown }).__TAURI_INTERNALS__
  root.dataset = {}
})

function withDesktopUserAgent(userAgent: string) {
  ;(globalThis as unknown as { __TAURI_INTERNALS__: { invoke: () => void } }).__TAURI_INTERNALS__ = {
    invoke: () => undefined,
  }
  Object.defineProperty(globalThis, 'navigator', {
    configurable: true,
    value: { userAgent },
  })
  Object.defineProperty(globalThis, 'document', {
    configurable: true,
    value: { documentElement: root },
  })
}

describe('desktopShell', () => {
  it('stays off in the browser', () => {
    expect(isDesktopShell()).toBe(false)
    expect(desktopPlatform()).toBe('web')
    markDesktopShell()
    expect(root.dataset.appShell).toBeUndefined()
  })

  it('marks the document when Tauri internals are present on macOS', () => {
    withDesktopUserAgent('Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)')
    expect(isDesktopShell()).toBe(true)
    expect(desktopPlatform()).toBe('macos')
    expect(isApplePlatform()).toBe(true)
    expect(primaryShortcutModifier()).toBe('⌘')
    markDesktopShell()
    expect(root.dataset.appShell).toBe('desktop')
    expect(root.dataset.platform).toBe('macos')
  })

  it('marks Windows desktop shell and exposes Ctrl shortcut modifier', () => {
    withDesktopUserAgent('Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36')
    expect(isDesktopShell()).toBe(true)
    expect(desktopPlatform()).toBe('windows')
    expect(isApplePlatform()).toBe(false)
    expect(primaryShortcutModifier()).toBe('Ctrl')
    markDesktopShell()
    expect(root.dataset.appShell).toBe('desktop')
    expect(root.dataset.platform).toBe('windows')
  })

  it('reads the workspace view from a native menu event', () => {
    expect(workspaceViewFromDesktopEvent(new Event('media-hub-navigate'))).toBeNull()
    expect(workspaceViewFromDesktopEvent(new CustomEvent('media-hub-navigate', { detail: { view: 'library' } }))).toBe('library')
  })
})
