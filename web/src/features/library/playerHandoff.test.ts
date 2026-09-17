import { describe, expect, it } from 'vitest'
import { handoffEnded, handoffStatusText } from './playerHandoff'

describe('playerHandoff', () => {
  it('names the mpv window and shows the clock once mpv reports a duration', () => {
    expect(handoffStatusText(true, 90, 3600)).toBe('正在 mpv 窗口播放 · 1:30 / 1:00:00')
    expect(handoffStatusText(false, 90, 3600)).toBe('已暂停 · 1:30 / 1:00:00')
    expect(handoffStatusText(true, 0, 0)).toBe('正在 mpv 窗口播放')
  })

  it('only ends the handoff after mpv was seen running', () => {
    expect(handoffEnded(false, false)).toBe(false)
    expect(handoffEnded(false, undefined)).toBe(false)
    expect(handoffEnded(true, undefined)).toBe(false)
    expect(handoffEnded(true, true)).toBe(false)
    expect(handoffEnded(true, false)).toBe(true)
  })
})
