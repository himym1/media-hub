import type { EmbyItem } from '../../shared/api/mediaHub'
import { episodeLabel } from './libraryPlayback'

export type PlayerQueueItem = {
  id: string
  title: string
}

export function playableLibraryQueue(items: EmbyItem[]): PlayerQueueItem[] {
  return items
    .filter((item) => item.type === 'Movie' || item.type === 'Video')
    .map((item) => ({ id: item.id, title: item.name.trim() || '未命名' }))
}

export function episodeQueue(items: EmbyItem[], seriesTitle = ''): PlayerQueueItem[] {
  return items.map((item) => ({
    id: item.id,
    title: episodeLabel(item, seriesTitle),
  }))
}

export function ensureQueueContains(queue: PlayerQueueItem[], current: PlayerQueueItem | null) {
  if (!current?.id) return queue
  if (queue.some((item) => item.id === current.id)) return queue
  return [current, ...queue]
}

export function nextQueueItem(queue: PlayerQueueItem[], currentId: string) {
  const index = queue.findIndex((item) => item.id === currentId)
  if (index < 0 || index + 1 >= queue.length) return undefined
  return queue[index + 1]
}

export function previousQueueItem(queue: PlayerQueueItem[], currentId: string) {
  const index = queue.findIndex((item) => item.id === currentId)
  if (index <= 0) return undefined
  return queue[index - 1]
}

export function nextQueueButtonLabel(next: PlayerQueueItem, episodes: boolean) {
  return episodes ? `下一集 ${next.title}` : `下一个 ${next.title}`
}

export function previousQueueButtonLabel(previous: PlayerQueueItem, episodes: boolean) {
  return episodes ? `上一集 ${previous.title}` : `上一个 ${previous.title}`
}
