import type { Candidate, DiscoveryItem, SearchIdentity } from '../../shared/api/mediaHub'

export type PipelineLane = 'all' | 'transfer' | 'download'

export function isDownloadable(candidate: Pick<Candidate, 'transferState' | 'sourceId'>) {
  return candidate.transferState === 'downloadable' || candidate.sourceId === 'moviepilot'
}

export function pipelineLabel(candidate: Pick<Candidate, 'transferState' | 'sourceId'>) {
  return isDownloadable(candidate) ? 'PT 下载' : '115 转存'
}

export function matchesPipeline(candidate: Pick<Candidate, 'transferState' | 'sourceId'>, lane: PipelineLane) {
  if (lane === 'all') return true
  return lane === 'download' ? isDownloadable(candidate) : !isDownloadable(candidate)
}

export function pickSearchIdentity(identities: SearchIdentity[] | undefined, candidate?: Pick<Candidate, 'tmdbId'> | null) {
  if (!identities?.length) return null
  if (candidate?.tmdbId) {
    const matched = identities.find((item) => item.tmdbId === candidate.tmdbId)
    if (matched) return matched
  }
  return identities[0] ?? null
}

export function discoverySearchQuery(item: Pick<DiscoveryItem, 'title' | 'year'>): string {
  const title = item.title.trim()
  if (!title) return ''
  return item.year > 0 ? `${title} ${item.year}` : title
}

export function discoveryFocusSubtitle(item: Pick<DiscoveryItem, 'year' | 'mediaType'>): string {
  const typeLabel = item.mediaType === 'series' ? '剧集' : '电影'
  return item.year > 0 ? `${item.year} · ${typeLabel}` : typeLabel
}

export function prioritizeDiscoveryResults(
  results: Candidate[],
  item: Pick<DiscoveryItem, 'tmdbId' | 'year' | 'mediaType'>,
): Candidate[] {
  if (results.length === 0) return results
  const wantSeries = item.mediaType === 'series'
  return [...results].sort((left, right) => score(right, item, wantSeries) - score(left, item, wantSeries) || left.title.localeCompare(right.title))
}

export function discoveryResultsHeading(searching: boolean, focusTitle: string): string {
  if (searching && focusTitle) return `正在查找《${focusTitle}》可获取版本…`
  if (searching) return '正在搜索…'
  if (focusTitle) return `《${focusTitle}》可获取版本`
  return '搜索结果'
}

export function formatCandidateSize(bytes: number) {
  if (bytes <= 0) return '大小未知'
  const gib = bytes / 1024 / 1024 / 1024
  return gib >= 1024 ? `${(gib / 1024).toFixed(1)} TB` : `${gib.toFixed(1)} GB`
}

export function episodeLabel(candidate: Candidate) {
  if (!candidate.season) return ''
  if (!candidate.episodeStart) return `S${candidate.season}`
  return candidate.episodeStart === candidate.episodeEnd
    ? `S${candidate.season}E${candidate.episodeStart}`
    : `S${candidate.season}E${candidate.episodeStart}-${candidate.episodeEnd}`
}

export function resolutionBadgeClass(res: string) {
  const lower = res.toLowerCase()
  if (lower.includes('2160') || lower.includes('4k')) return 'badge-res-4k'
  if (lower.includes('1080')) return 'badge-res-1080p'
  return 'badge-res-default'
}

export function dynamicRangeBadgeClass(hdr?: string) {
  if (!hdr) return ''
  const upper = hdr.toUpperCase()
  if (upper.includes('DV') || upper.includes('DOLBY')) return 'badge-hdr-dv'
  if (upper.includes('HDR')) return 'badge-hdr-standard'
  return 'badge-hdr-default'
}

function score(
  candidate: Candidate,
  item: Pick<DiscoveryItem, 'tmdbId' | 'year' | 'mediaType'>,
  wantSeries: boolean,
): number {
  let value = 0
  if (candidate.tmdbId && candidate.tmdbId === item.tmdbId) value += 100
  const isSeries = candidate.mediaType === 'series'
  if (wantSeries === isSeries) value += 20
  if (item.year > 0 && candidate.year === item.year) value += 10
  if (candidate.transferToken) value += 5
  if (candidate.transferState === 'available' || candidate.transferState === 'downloadable') value += 3
  return value
}
