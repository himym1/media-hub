import type { EmbyItem } from '../../shared/api/mediaHub'

export function playbackStatus(item: EmbyItem) {
  if (item.played) return '已看'
  const position = item.playbackPositionMs ?? 0
  if (position < 30_000) return '未开始'
  const totalSeconds = Math.floor(position / 1000)
  const hours = Math.floor(totalSeconds / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)
  const seconds = totalSeconds % 60
  const time = hours > 0
    ? `${hours}:${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`
    : `${minutes}:${String(seconds).padStart(2, '0')}`
  return `继续 ${time}`
}

export const embyOpenLabel = '在 Emby 打开'

export function playbackActionLabel(item: Pick<EmbyItem, 'played' | 'playbackPositionMs'>) {
  if (item.played) return '重新播放'
  if ((item.playbackPositionMs ?? 0) >= 30_000) return '继续播放'
  return '播放'
}

export function libraryWatchLabel(inPage: boolean, item: Pick<EmbyItem, 'played' | 'playbackPositionMs'>) {
  return inPage ? playbackActionLabel(item) : embyOpenLabel
}

export function episodeLabel(item: Pick<EmbyItem, 'name' | 'episode'>, seriesTitle = '') {
  const number = (item.episode ?? 0) > 0 ? `第 ${item.episode} 集` : null
  const title = item.name.trim() && item.name.trim() !== seriesTitle.trim() ? item.name.trim() : null
  return [number, title].filter(Boolean).join(' · ') || '分集'
}

export function formatPlaybackClock(seconds: number) {
  if (!Number.isFinite(seconds) || seconds < 0) return '0:00'
  const total = Math.floor(seconds)
  const hours = Math.floor(total / 3600)
  const minutes = Math.floor((total % 3600) / 60)
  const rest = total % 60
  return hours > 0
    ? `${hours}:${String(minutes).padStart(2, '0')}:${String(rest).padStart(2, '0')}`
    : `${minutes}:${String(rest).padStart(2, '0')}`
}

