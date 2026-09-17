import type { EmbyMediaTechSpecs } from '../../shared/api/mediaHub'

export type TechBadgeItem = {
  key: string
  label: string
  className: string
}

export function buildTechBadges(specs?: EmbyMediaTechSpecs | null): TechBadgeItem[] {
  if (!specs) return []

  const badges: TechBadgeItem[] = []

  if (specs.resolution) {
    badges.push({
      key: 'res',
      label: specs.resolution,
      className: 'tech-badge tech-badge-res',
    })
  }

  if (specs.videoRange) {
    const vr = specs.videoRange
    if (vr === 'Dolby Vision') {
      badges.push({
        key: 'dovi',
        label: 'VISION',
        className: 'tech-badge tech-badge-dovi',
      })
    } else if (vr === 'HDR10+' || vr === 'HDR10' || vr === 'HDR') {
      badges.push({
        key: 'hdr',
        label: vr,
        className: 'tech-badge tech-badge-hdr',
      })
    }
  }

  if (specs.audioProfile) {
    const ap = specs.audioProfile
    if (ap === 'Dolby Atmos') {
      badges.push({
        key: 'atmos',
        label: 'ATMOS',
        className: 'tech-badge tech-badge-atmos',
      })
    } else if (ap === 'DTS:X') {
      badges.push({
        key: 'dtsx',
        label: 'DTS:X',
        className: 'tech-badge tech-badge-atmos',
      })
    }
  }

  if (specs.audioCodec) {
    const channelPart = specs.audioChannels ? ` ${specs.audioChannels}` : ''
    badges.push({
      key: 'audio',
      label: `${specs.audioCodec}${channelPart}`,
      className: 'tech-badge tech-badge-audio',
    })
  }

  if (specs.videoCodec) {
    badges.push({
      key: 'vcodec',
      label: specs.videoCodec,
      className: 'tech-badge tech-badge-codec',
    })
  }

  if (specs.bitDepth && specs.bitDepth >= 10 && specs.videoRange !== 'Dolby Vision' && specs.videoRange !== 'HDR10') {
    badges.push({
      key: 'bitdepth',
      label: `${specs.bitDepth}-BIT`,
      className: 'tech-badge tech-badge-dim',
    })
  }

  if (specs.aspectRatio && specs.aspectRatio !== '16:9' && specs.aspectRatio !== '1.78:1') {
    badges.push({
      key: 'ar',
      label: specs.aspectRatio,
      className: 'tech-badge tech-badge-dim',
    })
  }

  return badges
}
