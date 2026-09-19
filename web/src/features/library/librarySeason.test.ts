import { describe, expect, it } from 'vitest'
import {
  seasonDisplayName,
  calculateEpisodeProgress,
  episodeDisplayNumber,
  sortSeasonEpisodes,
} from './librarySeason'

describe('librarySeason utils', () => {
  it('formats season display names correctly', () => {
    expect(seasonDisplayName(1)).toBe('第 1 季')
    expect(seasonDisplayName(2)).toBe('第 2 季')
    expect(seasonDisplayName(0)).toBe('特别篇')
    expect(seasonDisplayName(12)).toBe('第 12 季')
  })

  it('calculates episode playback progress percentage correctly', () => {
    // 0 progress
    expect(calculateEpisodeProgress(0, 45)).toBe(0)
    expect(calculateEpisodeProgress(undefined, 45)).toBe(0)

    // half played: 25 mins out of 50 mins
    expect(calculateEpisodeProgress(25 * 60 * 1000, 50)).toBe(50)

    // almost finished: 44 mins out of 45 mins
    expect(calculateEpisodeProgress(44 * 60 * 1000, 45)).toBe(98)

    // overflow bounded to 100%
    expect(calculateEpisodeProgress(60 * 60 * 1000, 45)).toBe(100)

    // unknown runtime returns minimum indicator
    expect(calculateEpisodeProgress(120_000, 0)).toBe(15)
  })

  it('formats episode numbers correctly', () => {
    expect(episodeDisplayNumber(1, 5)).toBe('第 5 集')
    expect(episodeDisplayNumber(2, 1)).toBe('第 1 集')
    expect(episodeDisplayNumber(1, 0)).toBe('分集')
  })

  it('sorts a season by episode number before reversing', () => {
    const items = [
      { id: 'e3', episode: 3 },
      { id: 'e1', episode: 1 },
      { id: 'e2', episode: 2 },
    ]
    expect(sortSeasonEpisodes(items).map((item) => item.id)).toEqual(['e1', 'e2', 'e3'])
    expect(sortSeasonEpisodes(items, true).map((item) => item.id)).toEqual(['e3', 'e2', 'e1'])
  })
})
