import { describe, expect, it } from 'vitest'
import type { EmbyItem } from '../../shared/api/mediaHub'
import {
  ensureQueueContains,
  episodeQueue,
  nextQueueButtonLabel,
  nextQueueItem,
  playableLibraryQueue,
} from './libraryPlaylist'

const item = (input: Partial<EmbyItem>): EmbyItem => ({
  id: 'id',
  name: 'Name',
  type: 'Movie',
  ...input,
})

describe('libraryPlaylist', () => {
  it('keeps movies and videos in library order and skips series folders', () => {
    expect(playableLibraryQueue([
      item({ id: 'a', name: 'Clip A', type: 'Video' }),
      item({ id: 's', name: 'Show', type: 'Series' }),
      item({ id: 'b', name: 'Clip B', type: 'Movie' }),
    ])).toEqual([
      { id: 'a', title: 'Clip A' },
      { id: 'b', title: 'Clip B' },
    ])
  })

  it('labels episode queues and picks the following item', () => {
    const queue = episodeQueue([
      item({ id: 'e1', name: '连接', episode: 1 }),
      item({ id: 'e2', name: '验收剧集', episode: 2 }),
    ], '验收剧集')
    expect(queue[0]?.title).toBe('第 1 集 · 连接')
    expect(nextQueueItem(queue, 'e1')?.id).toBe('e2')
    expect(nextQueueItem(queue, 'e2')).toBeUndefined()
    expect(nextQueueButtonLabel(queue[1]!, true)).toBe('下一集 第 2 集')
    expect(nextQueueButtonLabel({ id: 'a', title: 'Clip A' }, false)).toBe('下一个 Clip A')
  })

  it('inserts the playing item when it is outside the loaded page', () => {
    const queue = playableLibraryQueue([item({ id: 'b', name: 'B' })])
    expect(ensureQueueContains(queue, { id: 'a', title: 'A' })).toEqual([
      { id: 'a', title: 'A' },
      { id: 'b', title: 'B' },
    ])
    expect(ensureQueueContains(queue, { id: 'b', title: 'B' })).toEqual(queue)
  })
})
