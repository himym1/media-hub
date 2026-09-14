import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  cancelScheduledPlayerClick,
  clampPictureZoom,
  formatPictureZoom,
  isPlayerAspectId,
  logZoomFromLinear,
  nextPlayerAspect,
  playerAspectClassName,
  playerAspectModes,
  playerClickDelayMs,
  playbackRates,
  playbackSkipSeconds,
  schedulePlayerClick,
  shouldAutoHidePlayerChrome,
  shouldRevealChromeFromNativePointer,
  shouldArmNativeChromeHide,
  nativeEmbedRect,
  nativePointerFromStatus,
  playerChromeInsets,
  stepPictureZoom,
} from './playerChrome'

afterEach(() => {
  vi.useRealTimers()
})

describe('playerChrome', () => {
  it('keeps the same aspect labels as Android', () => {
    expect(playerAspectModes.map((mode) => mode.label)).toEqual([
      '自适应',
      '铺满',
      '宽度优先',
      '高度优先',
      '拉伸',
    ])
    expect(playerAspectModes.every((mode) => mode.description.length > 0)).toBe(true)
    expect(playerAspectClassName('zoom')).toBe('is-aspect-zoom')
    expect(isPlayerAspectId('fit')).toBe(true)
    expect(isPlayerAspectId('stretch')).toBe(false)
  })

  it('cycles aspect modes from 自适应', () => {
    expect(nextPlayerAspect('fit')).toBe('zoom')
    expect(nextPlayerAspect('fill')).toBe('fit')
  })

  it('clamps and formats picture zoom for the toolbar', () => {
    expect(clampPictureZoom(Number.NaN)).toBe(1)
    expect(clampPictureZoom(0.12)).toBe(0.5)
    expect(clampPictureZoom(9)).toBe(3)
    expect(stepPictureZoom(1, 1)).toBe(1.1)
    expect(stepPictureZoom(0.5, -1)).toBe(0.5)
    expect(formatPictureZoom(1)).toBe('100%')
    expect(formatPictureZoom(1.1)).toBe('110%')
    expect(logZoomFromLinear(2)).toBe(1)
  })

  it('hides chrome while playing unless it is pinned or failed', () => {
    expect(shouldAutoHidePlayerChrome(true, true, false, false)).toBe(true)
    expect(shouldAutoHidePlayerChrome(false, true, false, false)).toBe(true)
    expect(shouldAutoHidePlayerChrome(false, true, true, false)).toBe(false)
    expect(shouldAutoHidePlayerChrome(false, false, false, false)).toBe(false)
    expect(shouldAutoHidePlayerChrome(true, true, false, false, false)).toBe(false)
  })

  it('reveals chrome when the native pointer moves or enters', () => {
    expect(nativePointerFromStatus({ cursorHover: true })).toBeNull()
    const pointer = nativePointerFromStatus({ mouseX: 10, mouseY: 20, cursorHover: true })
    expect(shouldRevealChromeFromNativePointer(null, pointer!)).toBe(true)
    expect(shouldRevealChromeFromNativePointer(pointer, pointer!)).toBe(false)
    expect(shouldRevealChromeFromNativePointer(pointer, { x: 12, y: 20, hover: true })).toBe(true)
    expect(shouldRevealChromeFromNativePointer(pointer, { x: 10, y: 20, hover: false })).toBe(false)
    expect(shouldArmNativeChromeHide(null, pointer!)).toBe(false)
    expect(shouldArmNativeChromeHide(pointer, { x: 12, y: 20, hover: true })).toBe(true)
  })

  it('keeps native video out from under the chrome', () => {
    expect(playerChromeInsets(false, { getBoundingClientRect: () => ({ height: 64 }) }, { getBoundingClientRect: () => ({ height: 120 }) })).toEqual({ top: 0, bottom: 0 })
    expect(playerChromeInsets(true, { getBoundingClientRect: () => ({ height: 64 }) }, { getBoundingClientRect: () => ({ height: 120 }) })).toEqual({ top: 64, bottom: 120 })
    expect(nativeEmbedRect(
      { getBoundingClientRect: () => ({ left: 0, top: 0, width: 1280, height: 800 }) },
      { top: 64, bottom: 120 },
    )).toEqual({ x: 0, y: 64, width: 1280, height: 616 })
  })

  it('exposes skip and speed options used by the PC chrome', () => {
    expect(playbackSkipSeconds).toBe(10)
    expect(playbackRates).toContain(0.5)
    expect(playbackRates).toContain(1)
    expect(playerClickDelayMs).toBeGreaterThan(200)
  })

  it('delays a surface click so double-click can take over', () => {
    vi.useFakeTimers()
    const timer = { current: null as number | null }
    const click = vi.fn()
    schedulePlayerClick(timer, click)
    vi.advanceTimersByTime(200)
    expect(click).not.toHaveBeenCalled()
    cancelScheduledPlayerClick(timer)
    vi.runAllTimers()
    expect(click).not.toHaveBeenCalled()
    schedulePlayerClick(timer, click)
    vi.advanceTimersByTime(playerClickDelayMs)
    expect(click).toHaveBeenCalledOnce()
  })
})
