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
