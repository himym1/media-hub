import { describe, expect, it, vi } from 'vitest'
import { attachNativePlaybackSession, attachPlaybackSession } from './libraryPlaybackSession'

function fakeVideo(currentTime = 0) {
  const listeners = new Map<string, Set<() => void>>()
  const video = {
    currentTime,
    paused: false,
    addEventListener(type: string, handler: () => void) {
      const bucket = listeners.get(type) ?? new Set()
      bucket.add(handler)
      listeners.set(type, bucket)
    },
    removeEventListener(type: string, handler: () => void) {
      listeners.get(type)?.delete(handler)
    },
    dispatch(type: string) {
      for (const handler of listeners.get(type) ?? []) handler()
    },
  }
  return video
}

describe('attachPlaybackSession', () => {
  it('reports started then stopped on detach', () => {
    const report = vi.fn().mockResolvedValue(undefined)
    const video = fakeVideo(12.4)
    const detach = attachPlaybackSession('session-1', video as unknown as HTMLVideoElement, report)
    video.dispatch('playing')
    detach()
    expect(report.mock.calls.map((call) => call.slice(0, 3))).toEqual([
      ['session-1', 'started', 12400],
      ['session-1', 'stopped', 12400],
    ])
    expect(report.mock.calls[1][3]).toBe(true)
  })

  it('does not report without a session id', () => {
    const report = vi.fn()
    const detach = attachPlaybackSession(undefined, fakeVideo() as unknown as HTMLVideoElement, report)
    detach()
    expect(report).not.toHaveBeenCalled()
  })

  it('reports desktop native progress from the current clock', () => {
    vi.useFakeTimers()
    const report = vi.fn().mockResolvedValue(undefined)
    const state = { positionMs: 4_200, paused: false }
    const detach = attachNativePlaybackSession('session-2', () => state, report)
    expect(report.mock.calls[0]?.slice(0, 3)).toEqual(['session-2', 'started', 4200])
    state.positionMs = 19_500
    vi.advanceTimersByTime(15_000)
    expect(report.mock.calls[1]?.slice(0, 3)).toEqual(['session-2', 'progress', 19_500])
    detach()
    expect(report.mock.calls.at(-1)?.slice(0, 4)).toEqual(['session-2', 'stopped', 19_500, true])
    vi.useRealTimers()
  })
})
