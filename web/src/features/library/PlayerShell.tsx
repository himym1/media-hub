import { useEffect, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { CircleAlert } from 'lucide-react'
import { getEmbyEpisodes, getEmbyItem } from '../../shared/api/mediaHub'
import { canPlayNatively, closePlayerWindow } from '../../shared/desktop/nativePlayback'
import { commitUrl } from '../../shared/navigation/urlState'
import { LibraryPlayer } from './LibraryPlayer'
import './LibraryPlayer.css'
import { episodeLabel } from './libraryPlayback'
import { episodeQueue } from './libraryPlaylist'
import { playerItemId, playerSeriesId } from './playerRoute'

function locationPlayId() {
  return playerItemId()
}

function locationSeriesId() {
  return playerSeriesId()
}

export function PlayerShell() {
  const [playId, setPlayId] = useState<string | null>(locationPlayId)
  const [seriesId, setSeriesId] = useState<string | null>(locationSeriesId)
  const native = canPlayNatively()

  useEffect(() => {
    const restore = () => {
      setPlayId(locationPlayId())
      setSeriesId(locationSeriesId())
    }
    window.addEventListener('popstate', restore)
    return () => window.removeEventListener('popstate', restore)
  }, [])

  const item = useQuery({
    queryKey: ['emby-item', playId],
    queryFn: () => getEmbyItem(playId!),
    enabled: Boolean(playId),
  })
  const seriesKey = seriesId && seriesId !== playId ? seriesId : item.data?.type === 'Series' ? item.data.id : null
  const series = useQuery({
    queryKey: ['emby-item', seriesKey],
    queryFn: () => getEmbyItem(seriesKey!),
    enabled: Boolean(seriesKey),
  })
  const episodes = useQuery({
    queryKey: ['emby-episodes', seriesKey],
    queryFn: () => getEmbyEpisodes(seriesKey!),
    enabled: Boolean(seriesKey),
  })

  const playEpisode = episodes.data?.items.find((episode) => episode.id === playId)
  const seriesTitle = series.data?.name ?? (item.data?.type === 'Series' ? item.data.name : '')
  const title = playEpisode
    ? episodeLabel(playEpisode, seriesTitle)
    : item.data?.name ?? '播放'
  const externalUrl = playEpisode?.externalUrl ?? item.data?.externalUrl ?? ''
  const queue = playEpisode && episodes.data?.items.length
    ? episodeQueue(episodes.data.items, seriesTitle)
    : item.data && (item.data.type === 'Movie' || item.data.type === 'Video')
      ? [{ id: item.data.id, title: item.data.name }]
      : []

  useEffect(() => {
    if (title) document.title = title
  }, [title])

  const close = () => {
    void closePlayerWindow()
  }
  const startQueueItem = (id: string) => {
    if (id === playId) return
    setPlayId(id)
    commitUrl({ play: id }, 'replace')
  }

  if (!playId) {
    return (
      <div className="player-shell-status">
        <strong>没有可播放的条目</strong>
        <button className="secondary-command" onClick={close} type="button">关闭</button>
      </div>
    )
  }
  if (item.isLoading && !item.data) {
    return <div className="player-shell-status status-loading">正在打开播放器…</div>
  }
  if (item.isError) {
    return (
      <div className="player-shell-status">
        <div className="inline-error">
          <CircleAlert size={18} />
          <div><strong>无法读取播放条目</strong><span>{item.error.message}</span></div>
          <button onClick={() => void item.refetch()} type="button">重试</button>
        </div>
        <button className="secondary-command" onClick={close} type="button">关闭</button>
      </div>
    )
  }
  if (item.data?.type === 'Series' && !playEpisode) {
    return (
      <div className="player-shell-status">
        <strong>请选择要播放的分集</strong>
        <button className="secondary-command" onClick={close} type="button">关闭</button>
      </div>
    )
  }
  if (!native) {
    return (
      <div className="player-shell-status">
        <h1>{title}</h1>
        <p>请在桌面应用中播放，或在 Emby 打开。</p>
        {externalUrl ? (
          <a className="primary-action" href={externalUrl} rel="noreferrer" target="_blank">在 Emby 打开</a>
        ) : null}
      </div>
    )
  }

  return (
    <LibraryPlayer
      externalUrl={externalUrl}
      itemId={playId}
      key={playId}
      onClose={close}
      onSelectQueueItem={startQueueItem}
      queue={queue}
      queueIsEpisodes={Boolean(playEpisode)}
      title={title}
    />
  )
}
