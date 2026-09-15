import { useEffect, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { CircleAlert } from 'lucide-react'
import { getEmbyEpisodes, getEmbyItem } from '../../shared/api/mediaHub'
import { canPlayNatively, closePlayerWindow } from '../../shared/desktop/nativePlayback'
import { commitUrl } from '../../shared/navigation/urlState'
import { LibraryPlayer } from './LibraryPlayer'
import './LibraryPlayer.css'
import { episodeLabel } from './libraryPlayback'
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
  const nextEpisode = playEpisode
    ? episodes.data?.items.find((episode) => (
      episode.season === playEpisode.season && (episode.episode ?? 0) === (playEpisode.episode ?? 0) + 1
    ))
    : undefined

  useEffect(() => {
    if (title) document.title = title
  }, [title])

  const close = () => {
    void closePlayerWindow()
  }
  const startNext = () => {
    if (!nextEpisode) return
    setPlayId(nextEpisode.id)
    commitUrl({ play: nextEpisode.id }, 'replace')
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
      nextEpisodeLabel={nextEpisode ? episodeLabel(nextEpisode, seriesTitle) : undefined}
      onClose={close}
      onNextEpisode={nextEpisode ? startNext : undefined}
      title={title}
    />
  )
}
