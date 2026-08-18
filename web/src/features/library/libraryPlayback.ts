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
