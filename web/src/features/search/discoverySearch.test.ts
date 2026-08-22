import { describe, expect, it } from 'vitest'
import {
  discoveryFocusSubtitle,
  discoveryResultsHeading,
  discoverySearchQuery,
  prioritizeDiscoveryResults,
} from './discoverySearch'
import type { Candidate } from '../../shared/api/mediaHub'

describe('discoverySearch', () => {
  it('builds a year-aware query', () => {
    expect(discoverySearchQuery({ title: '头号玩家', year: 2018 })).toBe('头号玩家 2018')
    expect(discoverySearchQuery({ title: '范海辛', year: 0 })).toBe('范海辛')
  })

  it('ranks transferable tmdb matches first', () => {
    const ranked = prioritizeDiscoveryResults(
      [
        candidate({ id: 'a', title: 'Wrong', year: 2018, mediaType: 'movie', tmdbId: undefined, token: undefined }),
        candidate({ id: 'b', title: 'Ready Player One', year: 2018, mediaType: 'movie', tmdbId: '333339', token: 'tok' }),
        candidate({ id: 'c', title: 'Series Hit', year: 2018, mediaType: 'series', tmdbId: '333339', token: 'tok' }),
      ],
      { tmdbId: '333339', year: 2018, mediaType: 'movie' },
    )
    expect(ranked[0]?.id).toBe('b')
  })

  it('formats focus copy', () => {
    expect(discoveryFocusSubtitle({ year: 2018, mediaType: 'movie' })).toBe('2018 · 电影')
    expect(discoveryResultsHeading(true, '头号玩家')).toBe('正在查找《头号玩家》可转存版本…')
    expect(discoveryResultsHeading(false, '头号玩家')).toBe('《头号玩家》可转存版本')
  })
})

function candidate(input: {
  id: string
  title: string
  year: number
  mediaType: 'movie' | 'series'
  tmdbId?: string
  token?: string
}): Candidate {
  return {
    id: input.id,
    title: input.title,
    year: input.year,
    mediaType: input.mediaType,
    source: 'juying',
    sourceId: input.id,
    tmdbId: input.tmdbId,
    release: {
      resolution: '1080p',
      videoCodec: 'x264',
      sizeBytes: 0,
    },
    transferState: input.token ? 'available' : 'unavailable',
    transferToken: input.token,
  }
}
