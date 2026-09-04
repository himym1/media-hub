import { afterEach, describe, expect, it, vi } from 'vitest'
import { canPlayNatively, playNatively } from './nativePlayback'

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

  it('invokes play_native without throwing when Tauri is present', async () => {
    const invoke = vi.fn(async () => undefined)
    ;(globalThis as unknown as { __TAURI_INTERNALS__: { invoke: typeof invoke } }).__TAURI_INTERNALS__ = { invoke }
    await playNatively('https://cdn.example/movie.mkv', { title: '验收影片', startPositionMs: 1500 })
    expect(invoke).toHaveBeenCalledWith('play_native', {
      url: 'https://cdn.example/movie.mkv',
      title: '验收影片',
      startPositionMs: 1500,
    })
  })
})
