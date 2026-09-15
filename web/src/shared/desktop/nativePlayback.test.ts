import { afterEach, describe, expect, it, vi } from 'vitest'
import { attachNativeSubtitle, boundsFromElement, bytesToBase64, canPlayNatively, closePlayerWindow, controlNatively, nativeSubtitleFromBytes, openPlayerWindow, playNatively, setNativeCursorVisible, toggleNativeWindow } from './nativePlayback'

afterEach(() => {
  delete (globalThis as { __TAURI_INTERNALS__?: unknown }).__TAURI_INTERNALS__
})

describe('nativePlayback', () => {
  it('is off in the browser', () => {
    expect(canPlayNatively()).toBe(false)
  })

  it('is on when Tauri internals expose invoke', () => {
    ;(globalThis as unknown as { __TAURI_INTERNALS__: { invoke: () => Promise<void> } }).__TAURI_INTERNALS__ = {
      invoke: async () => undefined,
    }
    expect(canPlayNatively()).toBe(true)
  })

  it('invokes play_native with the embed bounds and subtitle', async () => {
    const invoke = vi.fn(async () => undefined)
    ;(globalThis as unknown as { __TAURI_INTERNALS__: { invoke: typeof invoke } }).__TAURI_INTERNALS__ = { invoke }
    const subtitle = nativeSubtitleFromBytes(new TextEncoder().encode('hello').buffer, 'chi.ass')
    await playNatively('https://cdn.example/movie.mkv', {
      title: '验收影片',
      startPositionMs: 1500,
      userAgent: 'Mozilla/5.0 MediaHub',
      bounds: { x: 10, y: 64, width: 1200, height: 640 },
      subtitle,
    })
    expect(invoke).toHaveBeenCalledWith('play_native', {
      url: 'https://cdn.example/movie.mkv',
      title: '验收影片',
      startPositionMs: 1500,
      userAgent: 'Mozilla/5.0 MediaHub',
      bounds: { x: 10, y: 64, width: 1200, height: 640 },
      subtitleBase64: bytesToBase64(new TextEncoder().encode('hello').buffer),
      subtitleFileName: 'chi.ass',
    })
  })

  it('attaches a sidecar after play has already started', async () => {
    const invoke = vi.fn(async () => undefined)
    ;(globalThis as unknown as { __TAURI_INTERNALS__: { invoke: typeof invoke } }).__TAURI_INTERNALS__ = { invoke }
    const subtitle = nativeSubtitleFromBytes(new TextEncoder().encode('hello').buffer, 'chi.ass')
    await attachNativeSubtitle(subtitle!)
    expect(invoke).toHaveBeenCalledWith('attach_native_subtitle', {
      subtitleBase64: bytesToBase64(new TextEncoder().encode('hello').buffer),
      subtitleFileName: 'chi.ass',
    })
  })

  it('reads bounds from an element box', () => {
    const element = {
      getBoundingClientRect: () => ({ left: 12, top: 48, width: 800, height: 450 }),
    } as unknown as Element
    expect(boundsFromElement(element)).toEqual({ x: 12, y: 48, width: 800, height: 450 })
  })

  it('surfaces a generic message when the native error contains a url', async () => {
    const invoke = vi.fn(async () => {
      throw 'failed https://cdn.example/movie.mkv'
    })
    ;(globalThis as unknown as { __TAURI_INTERNALS__: { invoke: typeof invoke } }).__TAURI_INTERNALS__ = { invoke }
    await expect(playNatively('https://cdn.example/movie.mkv', {
      title: '验收影片',
      bounds: { x: 0, y: 0, width: 100, height: 100 },
    })).rejects.toThrow('系统播放器未能打开这路流。')
  })

  it('keeps a safe native error string', async () => {
    const invoke = vi.fn(async () => {
      throw '未找到 mpv。请先安装 mpv 并确保在 PATH 中。'
    })
    ;(globalThis as unknown as { __TAURI_INTERNALS__: { invoke: typeof invoke } }).__TAURI_INTERNALS__ = { invoke }
    await expect(playNatively('https://cdn.example/movie.mkv', {
      title: '验收影片',
      bounds: { x: 0, y: 0, width: 100, height: 100 },
    })).rejects.toThrow('未找到 mpv。请先安装 mpv 并确保在 PATH 中。')
  })

  it('sends aspect and zoom through native_control', async () => {
    const invoke = vi.fn(async () => undefined)
    ;(globalThis as unknown as { __TAURI_INTERNALS__: { invoke: typeof invoke } }).__TAURI_INTERNALS__ = { invoke }
    await controlNatively('aspect', undefined, 'zoom')
    await controlNatively('zoom', 1.2)
    await controlNatively('cycle-audio')
    expect(invoke).toHaveBeenCalledWith('native_control', { action: 'aspect', value: undefined, mode: 'zoom' })
    expect(invoke).toHaveBeenCalledWith('native_control', { action: 'zoom', value: 1.2, mode: undefined })
    expect(invoke).toHaveBeenCalledWith('native_control', { action: 'cycle-audio', value: undefined, mode: undefined })
  })

  it('toggles the desktop window maximize state', async () => {
    const invoke = vi.fn(async () => undefined)
    ;(globalThis as unknown as { __TAURI_INTERNALS__: { invoke: typeof invoke } }).__TAURI_INTERNALS__ = { invoke }
    await toggleNativeWindow()
    expect(invoke).toHaveBeenCalledWith('toggle_native_window', {})
  })

  it('hides the desktop pointer without failing on older shells', async () => {
    const invoke = vi.fn(async () => undefined)
    ;(globalThis as unknown as { __TAURI_INTERNALS__: { invoke: typeof invoke } }).__TAURI_INTERNALS__ = { invoke }
    await setNativeCursorVisible(false)
    expect(invoke).toHaveBeenCalledWith('set_native_cursor_visible', { visible: false })
    invoke.mockRejectedValueOnce('missing')
    await expect(setNativeCursorVisible(true)).resolves.toBeUndefined()
  })

  it('opens a dedicated player window', async () => {
    const invoke = vi.fn(async () => undefined)
    ;(globalThis as unknown as { __TAURI_INTERNALS__: { invoke: typeof invoke } }).__TAURI_INTERNALS__ = { invoke }
    await openPlayerWindow({ playId: 'item-1', title: '验收影片', seriesId: 'series-2' })
    expect(invoke).toHaveBeenCalledWith('open_player_window', {
      playId: 'item-1',
      title: '验收影片',
      seriesId: 'series-2',
    })
  })

  it('closes the player window without throwing on older shells', async () => {
    const invoke = vi.fn(async () => undefined)
    ;(globalThis as unknown as { __TAURI_INTERNALS__: { invoke: typeof invoke } }).__TAURI_INTERNALS__ = { invoke }
    await closePlayerWindow()
    expect(invoke).toHaveBeenCalledWith('close_player_window', {})
    invoke.mockRejectedValueOnce('missing')
    await expect(closePlayerWindow()).resolves.toBeUndefined()
  })
})
