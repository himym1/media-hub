import { ArrowUpDown, Check, CircleAlert, Play } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getEmbyEpisodes, type EmbyEpisode } from '../../shared/api/mediaHub'
import { LibraryWatchAction } from './LibraryWatchAction'
import { episodeLabel, playbackStatus } from './libraryPlayback'

type LibraryEpisodesProps = {
  seriesId: string
  seriesTitle: string
  inPagePlayback: boolean
  onPlay: (episode: EmbyEpisode) => void
}

export function LibraryEpisodes({ seriesId, seriesTitle, inPagePlayback, onPlay }: LibraryEpisodesProps) {
  const episodes = useQuery({
    queryKey: ['emby-episodes', seriesId],
    queryFn: () => getEmbyEpisodes(seriesId),
  })
  const rawItems = episodes.data?.items
  const items = useMemo(() => rawItems ?? [], [rawItems])
  const [descending, setDescending] = useState(false)

  const upNextEpisode = useMemo(() => {
    if (items.length === 0) return null
    return (
      items.find((ep) => !ep.played && (ep.playbackPositionMs ?? 0) >= 30_000)
      ?? items.find((ep) => !ep.played)
      ?? items[0]
    )
  }, [items])

  const displayItems = useMemo(() => {
    if (!descending) return items
    return [...items].reverse()
  }, [items, descending])

  if (episodes.isLoading) return <div className="status-loading">正在读取分集…</div>
  if (episodes.isError) {
    return (
      <div className="inline-error">
        <CircleAlert size={18} />
        <div><strong>分集读取失败</strong><span>{episodes.error.message}</span></div>
        <button onClick={() => void episodes.refetch()} type="button">重试</button>
      </div>
    )
  }
  if (items.length === 0) {
    return <p className="library-subtitle-hint">Emby 暂未提供可播放分集</p>
  }

  return (
    <div className="library-episodes-container">
      {upNextEpisode ? (
        <div className="episode-up-next-card">
          <div className="up-next-header">
            <span className="up-next-tag">
              <Play fill="currentColor" size={11} />
              {upNextEpisode.played
                ? '重新观看'
                : (upNextEpisode.playbackPositionMs ?? 0) >= 30_000
                  ? '继续观看'
                  : '接续观看'}
            </span>
            <span className="up-next-status-text">{playbackStatus(upNextEpisode)}</span>
          </div>
          <div className="up-next-body">
            <strong className="up-next-title">{episodeLabel(upNextEpisode, seriesTitle)}</strong>
            <LibraryWatchAction
              externalUrl={upNextEpisode.externalUrl}
              inPage={inPagePlayback}
              item={upNextEpisode}
              name={episodeLabel(upNextEpisode, seriesTitle)}
              named
              onPlay={() => onPlay(upNextEpisode)}
            />
          </div>
        </div>
      ) : null}

      <div className="episode-controls-bar">
        <span className="episode-count-badge">共 {items.length} 集</span>
        {items.length > 6 ? (
          <button
            className="episode-sort-btn"
            onClick={() => setDescending((prev) => !prev)}
            type="button"
          >
            <ArrowUpDown size={12} />
            {descending ? '倒序（最新在首）' : '正序（第 1 集在首）'}
          </button>
        ) : null}
      </div>

      <ul className="library-episode-list">
        {displayItems.map((episode) => {
          const name = episodeLabel(episode, seriesTitle)
          const isPlayed = Boolean(episode.played)
          const hasProgress = (episode.playbackPositionMs ?? 0) >= 30_000 && !isPlayed
          return (
            <li className={isPlayed ? 'episode-item played' : hasProgress ? 'episode-item in-progress' : 'episode-item'} key={episode.id}>
              <div className="episode-meta-cell">
                <div className="episode-title-row">
                  {isPlayed ? (
                    <span aria-label="已看完" className="episode-check-badge" title="已看完">
                      <Check size={12} />
                    </span>
                  ) : null}
                  <strong>{name}</strong>
                </div>
                <small>{playbackStatus(episode)}</small>
              </div>
              <LibraryWatchAction
                externalUrl={episode.externalUrl}
                inPage={inPagePlayback}
                item={episode}
                name={name}
                named
                onPlay={() => onPlay(episode)}
              />
            </li>
          )
        })}
      </ul>
    </div>
  )
}
