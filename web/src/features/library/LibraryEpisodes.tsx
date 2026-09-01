import { CircleAlert, Play } from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
import { getEmbyEpisodes, type EmbyEpisode } from '../../shared/api/mediaHub'
import { episodeLabel, playbackActionLabel, playbackStatus } from './libraryPlayback'

type LibraryEpisodesProps = {
  seriesId: string
  seriesTitle: string
  onPlay: (episode: EmbyEpisode) => void
}

export function LibraryEpisodes({ seriesId, seriesTitle, onPlay }: LibraryEpisodesProps) {
  const episodes = useQuery({
    queryKey: ['emby-episodes', seriesId],
    queryFn: () => getEmbyEpisodes(seriesId),
  })
  const items = episodes.data?.items ?? []

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
    <ul className="library-episode-list">
      {items.map((episode) => {
        const label = playbackActionLabel(episode)
        const name = episodeLabel(episode, seriesTitle)
        return (
          <li key={episode.id}>
            <div>
              <strong>{name}</strong>
              <small>{playbackStatus(episode)}</small>
            </div>
            <button
              aria-label={`${label} ${name}`}
              className="primary-action"
              onClick={() => onPlay(episode)}
              type="button"
            >
              <Play size={16} />
              {label}
            </button>
          </li>
        )
      })}
    </ul>
  )
}
