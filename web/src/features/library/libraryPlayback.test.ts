import { describe, expect, it } from 'vitest'
import type { EmbyItem } from '../../shared/api/mediaHub'
import {
  canDirectPlayItem,
  episodeLabel,
  libraryWatchLabel,
  playbackActionLabel,
  playbackProgressPercent,
  playbackStatus,
} from './libraryPlayback'

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

  it('labels play actions like Android', () => {
    expect(playbackActionLabel(item())).toBe('播放')
    expect(playbackActionLabel(item({ playbackPositionMs: 30_000 }))).toBe('继续播放')
    expect(playbackActionLabel(item({ played: true }))).toBe('重新播放')
    expect(libraryWatchLabel(true, item())).toBe('播放')
    expect(libraryWatchLabel(false, item({ playbackPositionMs: 30_000 }))).toBe('在 Emby 打开')
  })

  it('labels episodes with a number and title', () => {
    expect(episodeLabel(item({ name: '连接', episode: 1 }), '验收剧集')).toBe('第 1 集 · 连接')
    expect(episodeLabel(item({ name: '验收剧集', episode: 2 }), '验收剧集')).toBe('第 2 集')
  })

  it('computes playback progress from real runtime and falls back safely', () => {
    expect(playbackProgressPercent(item())).toBe(0)
    expect(playbackProgressPercent(item({ played: true }))).toBe(100)
    // 24 分钟动漫看到 12 分钟应接近 50%，不再按 2 小时估算
    expect(playbackProgressPercent(item({ type: 'Episode', playbackPositionMs: 12 * 60_000, runtimeMs: 24 * 60_000 }))).toBe(50)
    // 无 runtimeMs 时仍给出合理区间
    expect(playbackProgressPercent(item({ type: 'Movie', playbackPositionMs: 55 * 60_000 }))).toBe(50)
    expect(playbackProgressPercent(item({ type: 'Series', playbackPositionMs: 15 * 60_000 }))).toBe(33)
    expect(playbackProgressPercent(item({ playbackPositionMs: 10_000, runtimeMs: 6_000_000 }))).toBe(4)
  })

  it('allows direct play only for movies and videos inside the desktop shell', () => {
    expect(canDirectPlayItem(true, item({ type: 'Movie' }))).toBe(true)
    expect(canDirectPlayItem(true, item({ type: 'Video' }))).toBe(true)
    expect(canDirectPlayItem(true, item({ type: 'Series' }))).toBe(false)
    expect(canDirectPlayItem(false, item({ type: 'Movie' }))).toBe(false)
  })
})
