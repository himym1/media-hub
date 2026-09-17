import { describe, expect, it } from 'vitest'
import { buildTechBadges } from './libraryTechSpecs'

describe('buildTechBadges', () => {
  it('returns empty array when specs are missing or null', () => {
    expect(buildTechBadges(null)).toEqual([])
    expect(buildTechBadges(undefined)).toEqual([])
    expect(buildTechBadges({})).toEqual([])
  })

  it('builds full Infuse-grade badges for 4K Dolby Vision Atmos media', () => {
    const badges = buildTechBadges({
      resolution: '4K UHD',
      videoRange: 'Dolby Vision',
      videoCodec: 'HEVC',
      audioProfile: 'Dolby Atmos',
      audioCodec: 'TrueHD',
      audioChannels: '7.1',
      aspectRatio: '2.39:1',
      bitDepth: 10,
    })

    expect(badges.map((b) => b.label)).toEqual([
      '4K UHD',
      'VISION',
      'ATMOS',
      'TrueHD 7.1',
      'HEVC',
      '2.39:1',
    ])

    const doviBadge = badges.find((b) => b.label === 'VISION')
    expect(doviBadge?.className).toBe('tech-badge tech-badge-dovi')

    const atmosBadge = badges.find((b) => b.label === 'ATMOS')
    expect(atmosBadge?.className).toBe('tech-badge tech-badge-atmos')

    const resBadge = badges.find((b) => b.label === '4K UHD')
    expect(resBadge?.className).toBe('tech-badge tech-badge-res')
  })

  it('builds HDR10+ and DTS:X badges correctly', () => {
    const badges = buildTechBadges({
      resolution: '1080p',
      videoRange: 'HDR10+',
      videoCodec: 'H.264',
      audioProfile: 'DTS:X',
      audioCodec: 'DTS-HD MA',
      audioChannels: '5.1',
    })

    expect(badges.map((b) => b.label)).toEqual([
      '1080p',
      'HDR10+',
      'DTS:X',
      'DTS-HD MA 5.1',
      'H.264',
    ])

    const hdrBadge = badges.find((b) => b.label === 'HDR10+')
    expect(hdrBadge?.className).toBe('tech-badge tech-badge-hdr')
  })
})
