import { ArrowUpDown, Check, CircleAlert, Film, Play, Star } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { embyPrimaryImageURL, getEmbyEpisodes, type EmbyEpisode } from '../../shared/api/mediaHub'
import { LibraryWatchAction } from './LibraryWatchAction'
import { episodeLabel, playbackStatus } from './libraryPlayback'
import { calculateEpisodeProgress, seasonDisplayName } from './librarySeason'

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

  // 提取所有不同的季度
  const seasons = useMemo(() => {
    const set = new Set<number>()
    for (const ep of items) {
      set.add(ep.season ?? 1)
    }
    return Array.from(set).sort((a, b) => a - b)
  }, [items])

  const upNextEpisode = useMemo(() => {
    if (items.length === 0) return null
    return (
      items.find((ep) => !ep.played && (ep.playbackPositionMs ?? 0) >= 30_000)
      ?? items.find((ep) => !ep.played)
      ?? items[0]
    )
  }, [items])

  // 默认激活“接着看”所在季，或者第 1 季
  const [activeSeason, setActiveSeason] = useState<number>(1)

  useEffect(() => {
    if (seasons.length > 0) {
      const targetSeason = upNextEpisode?.season ?? seasons[0]
      setActiveSeason(seasons.includes(targetSeason) ? targetSeason : seasons[0])
    }
  }, [seasons, upNextEpisode?.season])

  // 当前季的分集列表
  const seasonItems = useMemo(() => {
    const list = items.filter((ep) => (ep.season ?? 1) === activeSeason)
    return descending ? [...list].reverse() : list
  }, [items, activeSeason, descending])

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
      {/* 待播 / 接着看卡片 */}
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
            {upNextEpisode.techSpecs?.resolution ? (
              <span className="tech-badge tech-badge-res" style={{ height: '20px', fontSize: '10px', padding: '0 6px' }}>
                {upNextEpisode.techSpecs.resolution}
              </span>
            ) : null}
            {upNextEpisode.techSpecs?.videoRange && upNextEpisode.techSpecs.videoRange !== 'SDR' ? (
              <span
                className={upNextEpisode.techSpecs.videoRange === 'Dolby Vision' ? 'tech-badge tech-badge-dovi' : 'tech-badge tech-badge-hdr'}
                style={{ height: '20px', fontSize: '10px', padding: '0 6px' }}
              >
                {upNextEpisode.techSpecs.videoRange === 'Dolby Vision' ? 'VISION' : upNextEpisode.techSpecs.videoRange}
              </span>
            ) : null}
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

      {/* 分季药丸切换器 (Infuse / 网易爆米花风格) */}
      {seasons.length > 1 ? (
        <div aria-label="剧集分季选择" className="season-tabs-bar" role="tablist">
          {seasons.map((seasonNum) => {
            const isActive = seasonNum === activeSeason
            const count = items.filter((ep) => (ep.season ?? 1) === seasonNum).length
            return (
              <button
                aria-selected={isActive}
                className={`season-pill-btn ${isActive ? 'active' : ''}`}
                key={seasonNum}
                onClick={() => setActiveSeason(seasonNum)}
                role="tab"
                type="button"
              >
                <span>{seasonDisplayName(seasonNum)}</span>
                <span className="season-ep-count">{count} 集</span>
              </button>
            )
          })}
        </div>
      ) : null}

      {/* 控制栏：集数统计与正倒序切换 */}
      <div className="episode-controls-bar">
        <span className="episode-count-badge">
          {seasons.length > 1 ? `${seasonDisplayName(activeSeason)} · ` : ''}共 {seasonItems.length} 集
        </span>
        {seasonItems.length > 4 ? (
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

      {/* 16:9 画幅剧照卡片流 (Infuse / 网易爆米花级) */}
      <ul className="library-episode-card-list">
        {seasonItems.map((episode) => (
          <EpisodeStillCard
            episode={episode}
            inPagePlayback={inPagePlayback}
            key={episode.id}
            onPlay={onPlay}
            seriesTitle={seriesTitle}
          />
        ))}
      </ul>
    </div>
  )
}

function EpisodeStillCard({
  episode,
  seriesTitle,
  inPagePlayback,
  onPlay,
}: {
  episode: EmbyEpisode
  seriesTitle: string
  inPagePlayback: boolean
  onPlay: (episode: EmbyEpisode) => void
}) {
  const [thumbFailed, setThumbFailed] = useState(false)
  const isPlayed = Boolean(episode.played)
  const hasProgress = (episode.playbackPositionMs ?? 0) >= 30_000 && !isPlayed
  const progressPct = calculateEpisodeProgress(episode.playbackPositionMs, episode.runtimeMinutes)
  const name = episodeLabel(episode, seriesTitle)
  const customTitle = episode.name?.trim()

  return (
    <li className={`episode-card-item ${isPlayed ? 'played' : ''} ${hasProgress ? 'in-progress' : ''}`}>
      {/* 16:9 剧照画幅缩略图 */}
      <div
        className="episode-still-thumb-wrap"
        onClick={() => onPlay(episode)}
        onKeyDown={(e) => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault()
            onPlay(episode)
          }
        }}
        role="button"
        tabIndex={0}
        title={`点击直接播放 ${name}`}
      >
        {!thumbFailed ? (
          <img
            alt=""
            className="episode-still-img"
            decoding="async"
            height={90}
            loading="lazy"
            onError={() => setThumbFailed(true)}
            src={embyPrimaryImageURL(episode.id)}
            width={160}
          />
        ) : (
          <div aria-hidden="true" className="episode-still-fallback">
            <Film className="episode-still-fallback-icon" size={24} />
          </div>
        )}

        {/* 悬浮直接播放图层 */}
        <div aria-hidden="true" className="episode-still-play-overlay">
          <span className="episode-play-circle">
            <Play fill="currentColor" size={14} />
          </span>
        </div>

        {/* 已完播标识 */}
        {isPlayed ? (
          <span aria-label="已看完" className="episode-still-badge-played" title="已看完">
            <Check size={11} />
            <span>已看</span>
          </span>
        ) : null}

        {/* 底部播放进度条 */}
        {hasProgress && progressPct > 0 ? (
          <div className="episode-still-progress-track">
            <div className="episode-still-progress-fill" style={{ width: `${progressPct}%` }} />
          </div>
        ) : null}
      </div>

      {/* 分集详情与操作 */}
      <div className="episode-card-body">
        <div className="episode-card-header">
          <div className="episode-card-title-group">
            <strong className="episode-card-number">
              第 {episode.episode || 1} 集
            </strong>
            {customTitle ? (
              <span className="episode-card-name" title={customTitle}>
                {customTitle}
              </span>
            ) : null}
          </div>

          <div className="episode-card-meta-pills">
            {episode.techSpecs?.resolution ? (
              <span className="episode-spec-pill">
                {episode.techSpecs.resolution}
                {episode.techSpecs.videoRange && episode.techSpecs.videoRange !== 'SDR'
                  ? ` · ${episode.techSpecs.videoRange === 'Dolby Vision' ? 'DV' : episode.techSpecs.videoRange}`
                  : ''}
              </span>
            ) : null}
            {episode.runtimeMinutes ? (
              <span className="episode-runtime-pill">
                {episode.runtimeMinutes} 分钟
              </span>
            ) : null}
            {episode.communityRating ? (
              <span className="episode-rating-pill">
                <Star className="rating-star" size={11} />
                <span>{episode.communityRating.toFixed(1)}</span>
              </span>
            ) : null}
          </div>
        </div>

        {episode.overview ? (
          <p className="episode-card-overview" title={episode.overview}>
            {episode.overview}
          </p>
        ) : null}

        <div className="episode-card-footer">
          <span className="episode-card-status">{playbackStatus(episode)}</span>
          <LibraryWatchAction
            externalUrl={episode.externalUrl}
            inPage={inPagePlayback}
            item={episode}
            name={name}
            named
            onPlay={() => onPlay(episode)}
          />
        </div>
      </div>
    </li>
  )
}
