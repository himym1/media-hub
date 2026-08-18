import { describe, expect, it } from 'vitest'
import type { EmbyItem } from '../../shared/api/mediaHub'
import { playbackStatus } from './libraryPlayback'

const item = (input: Partial<EmbyItem> = {}): EmbyItem => ({
  id: 'item-1',
  name: 'Movie',
  type: 'Movie',
  ...input,
})

describe('playbackStatus', () => {
  it('distinguishes new, resumable, and watched media', () => {
    expect(playbackStatus(item())).toBe('未开始')
    expect(playbackStatus(item({ playbackPositionMs: 2_500_000 }))).toBe('继续 41:40')
    expect(playbackStatus(item({ playbackPositionMs: 3_725_000 }))).toBe('继续 1:02:05')
    expect(playbackStatus(item({ played: true, playbackPositionMs: 0 }))).toBe('已看')
  })
})
