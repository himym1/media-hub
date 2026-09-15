import type { Candidate, DiscoveryItem } from '../../shared/api/mediaHub'

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
