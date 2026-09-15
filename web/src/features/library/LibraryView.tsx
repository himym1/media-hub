import { useEffect, useMemo, useState, type FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowDownUp, BookOpen, Captions, ChevronLeft, ChevronRight, CircleAlert, ExternalLink, Film, Play, RefreshCw, Search, Star, Trash2, X } from 'lucide-react'
import {
  deleteEmbyItem,
  downloadEmbyRemoteSubtitle,
  embyPrimaryImageURL,
  fetchLocalSubtitle,
  getEmbyEpisodes,
  getEmbyItem,
  getEmbyLibraries,
  getEmbyLibraryItems,
  previewEmbyItemDelete,
  refreshEmbyItem,
  refreshEmbyLibrary,
  searchEmbyItems,
  searchEmbyRemoteSubtitles,
  type EmbyDeletePreview,
  type EmbyEpisode,
  type EmbyItem,
  type EmbyItemDetail,
  type EmbyRemoteSubtitle,
} from '../../shared/api/mediaHub'
import { canPlayNatively, openPlayerWindow } from '../../shared/desktop/nativePlayback'
import { commitUrl } from '../../shared/navigation/urlState'
import { IconButton } from '../../shared/ui/IconButton'
import { LibraryEpisodes } from './LibraryEpisodes'
import { LibraryPlayer } from './LibraryPlayer'
import { LibraryWatchAction } from './LibraryWatchAction'
import { episodeLabel, playbackStatus } from './libraryPlayback'

const pageSize = 24

function locationValue(name: string) {
  return new URLSearchParams(window.location.search).get(name)
}

export function LibraryView() {
  const queryClient = useQueryClient()
  const [libraryId, setLibraryId] = useState<string | null>(() => locationValue('library'))
  const [itemId, setItemId] = useState<string | null>(() => locationValue('media'))
  const [playId, setPlayId] = useState<string | null>(() => locationValue('play'))
  const [page, setPage] = useState(0)
  const [queryText, setQueryText] = useState('')
  const [submittedQuery, setSubmittedQuery] = useState('')
  const [sortBy, setSortBy] = useState<'default' | 'year-desc' | 'year-asc' | 'name-asc'>('default')
  const [typeFilter, setTypeFilter] = useState<'all' | 'Movie' | 'Series'>('all')
  const [statusFilter, setStatusFilter] = useState<'all' | 'in-progress' | 'unplayed' | 'played'>('all')
  const [spotlightIndex, setSpotlightIndex] = useState(0)

  const libraries = useQuery({ queryKey: ['emby-libraries'], queryFn: getEmbyLibraries })
  const allLibraries = useMemo(() => libraries.data?.libraries ?? [], [libraries.data?.libraries])
  const libraryItems = useQuery({
    queryKey: ['emby-library-items', libraryId, page],
    queryFn: () => getEmbyLibraryItems(libraryId!, page * pageSize, pageSize),
    enabled: Boolean(libraryId) && !submittedQuery,
  })
  const search = useQuery({
    queryKey: ['emby-search', submittedQuery],
    queryFn: () => searchEmbyItems(submittedQuery, 100),
    enabled: Boolean(submittedQuery),
  })
  const detail = useQuery({
    queryKey: ['emby-item', itemId],
    queryFn: () => getEmbyItem(itemId!),
    enabled: Boolean(itemId),
  })
  const episodes = useQuery({
    queryKey: ['emby-episodes', itemId],
    queryFn: () => getEmbyEpisodes(itemId!),
    enabled: Boolean(itemId) && detail.data?.type === 'Series',
  })
  const refreshLibrary = useMutation({
    mutationFn: refreshEmbyLibrary,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['emby-library-items', libraryId] })
    },
  })
  const refreshItem = useMutation({
    mutationFn: refreshEmbyItem,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['emby-item', itemId] })
      await queryClient.invalidateQueries({ queryKey: ['emby-library-items'] })
      await queryClient.invalidateQueries({ queryKey: ['emby-search'] })
    },
  })

  const inPagePlayback = canPlayNatively()

  useEffect(() => {
    const restoreLocation = () => {
      const nextLibrary = locationValue('library')
      setLibraryId(nextLibrary)
      setItemId(locationValue('media'))
      setPlayId(locationValue('play'))
      setPage(0)
      setQueryText('')
      setSubmittedQuery('')
    }
    window.addEventListener('popstate', restoreLocation)
    return () => window.removeEventListener('popstate', restoreLocation)
  }, [])

  useEffect(() => {
    if (inPagePlayback || !playId) return
    setPlayId(null)
    commitUrl({ play: null }, 'replace')
  }, [inPagePlayback, playId])

  useEffect(() => {
    if (!libraries.data) return
    if (libraryId && allLibraries.some((library) => library.id === libraryId)) return
    const fallback = allLibraries[0]?.id ?? null
    setLibraryId(fallback)
    setItemId(null)
    setPlayId(null)
    setPage(0)
    commitUrl({ library: fallback, media: null, play: null }, 'replace')
  }, [libraries.data, libraryId, allLibraries])

  const selectLibrary = (id: string) => {
    setLibraryId(id)
    setItemId(null)
    setPlayId(null)
    setPage(0)
    setQueryText('')
    setSubmittedQuery('')
    setSpotlightIndex(0)
    commitUrl({ library: id, media: null, play: null })
  }
  const selectItem = (id: string) => {
    setItemId(id)
    setPlayId(null)
    commitUrl({ media: id, play: null })
  }
  const closeItem = () => {
    setItemId(null)
    setPlayId(null)
    commitUrl({ media: null, play: null })
  }
  const startPlay = (target: Pick<EmbyEpisode, 'id' | 'name' | 'externalUrl'>) => {
    if (!inPagePlayback) return
    const seriesId = detail.data?.type === 'Series' ? detail.data.id : undefined
    void openPlayerWindow({ playId: target.id, title: target.name, seriesId }).catch(() => {
      setPlayId(target.id)
      commitUrl({ play: target.id })
    })
  }
  const closePlay = () => {
    setPlayId(null)
    commitUrl({ play: null })
  }
  const submitSearch = (event: FormEvent) => {
    event.preventDefault()
    const value = queryText.trim()
    setSubmittedQuery(value)
    setItemId(null)
    setPlayId(null)
    setSpotlightIndex(0)
    commitUrl({ media: null, play: null })
  }
  const clearSearch = () => {
    setQueryText('')
    setSubmittedQuery('')
    setItemId(null)
    setPlayId(null)
    setSpotlightIndex(0)
    commitUrl({ media: null, play: null })
  }
  const changePage = (nextPage: number) => {
    setPage(nextPage)
    setItemId(null)
    setPlayId(null)
    setSpotlightIndex(0)
    commitUrl({ media: null, play: null })
  }

  const result = submittedQuery ? search : libraryItems
  const rawItems = useMemo(() => result.data?.items ?? [], [result.data?.items])

  const continueWatchingItems = useMemo(() => {
    return rawItems.filter((item) => (item.playbackPositionMs ?? 0) >= 30_000 && !item.played)
  }, [rawItems])

  const spotlightCandidates = useMemo(() => {
    if (rawItems.length === 0) return []
    const inProgress = rawItems.filter((item) => (item.playbackPositionMs ?? 0) >= 30_000 && !item.played)
    const rest = rawItems.filter((item) => !((item.playbackPositionMs ?? 0) >= 30_000 && !item.played))
    return [...inProgress, ...rest].slice(0, 4)
  }, [rawItems])

  const activeSpotlight = spotlightCandidates[spotlightIndex] ?? spotlightCandidates[0]

  const items = useMemo(() => {
    let list = [...rawItems]
    if (typeFilter !== 'all') {
      list = list.filter((item) => item.type === typeFilter)
    }
    if (statusFilter === 'in-progress') {
      list = list.filter((item) => (item.playbackPositionMs ?? 0) >= 30_000 && !item.played)
    } else if (statusFilter === 'played') {
      list = list.filter((item) => Boolean(item.played))
    } else if (statusFilter === 'unplayed') {
      list = list.filter((item) => !item.played && (item.playbackPositionMs ?? 0) < 30_000)
    }

    if (sortBy === 'year-desc') {
      list.sort((a, b) => (b.year ?? 0) - (a.year ?? 0))
    } else if (sortBy === 'year-asc') {
      list.sort((a, b) => (a.year ?? 9999) - (b.year ?? 9999))
    } else if (sortBy === 'name-asc') {
      list.sort((a, b) => a.name.localeCompare(b.name, 'zh-CN'))
    }
    return list
  }, [rawItems, typeFilter, statusFilter, sortBy])

  const total = submittedQuery ? items.length : (result.data?.total ?? 0)
  const hasActiveFilters = typeFilter !== 'all' || statusFilter !== 'all' || sortBy !== 'default'
  const resetFilters = () => {
    setTypeFilter('all')
    setStatusFilter('all')
    setSortBy('default')
  }
  const selectedLibrary = allLibraries.find((library) => library.id === libraryId)
  const mutationError = refreshLibrary.error ?? refreshItem.error
  const playEpisode = episodes.data?.items.find((episode) => episode.id === playId)
  const playTarget = playId && detail.data && detail.data.type !== 'Series' && detail.data.id === playId
    ? { id: detail.data.id, title: detail.data.name, externalUrl: detail.data.externalUrl }
    : playEpisode
      ? { id: playEpisode.id, title: episodeLabel(playEpisode, detail.data?.name ?? ''), externalUrl: playEpisode.externalUrl }
      : null
  const nextEpisode = playEpisode
    ? episodes.data?.items.find((episode) => episode.season === playEpisode.season && (episode.episode ?? 0) === (playEpisode.episode ?? 0) + 1)
    : undefined

  return (
    <section className={itemId ? 'library-page has-detail' : 'library-page'}>
      <header className="view-header compact-view-header">
        <div><h1>媒体库</h1><p>已入库的电影和剧集。</p></div>
        <IconButton label="刷新媒体库列表" onClick={() => void libraries.refetch()} subtle><RefreshCw size={17} /></IconButton>
      </header>

      {libraries.isError ? <div className="inline-error"><CircleAlert size={18} /><div><strong>媒体库读取失败</strong><span>{libraries.error.message}</span></div><button onClick={() => void libraries.refetch()} type="button">重试</button></div> : null}
      {mutationError ? <div className="source-warning error" role="alert"><CircleAlert size={16} /><span>{mutationError.message}</span></div> : null}

      <div className="library-browse">
        <div className="library-header-bar">
          <div className="library-nav-cluster">
            <nav aria-label="Emby 媒体库" className="library-tabs">
              {allLibraries.map((library) => {
                const typeLabel = libraryCollectionLabel(library.collectionType)
                const name = library.name
                const isSelected = library.id === libraryId
                return (
                  <button
                    aria-pressed={isSelected}
                    className={isSelected ? 'library-tab-pill active' : 'library-tab-pill'}
                    key={library.id}
                    onClick={() => selectLibrary(library.id)}
                    type="button"
                  >
                    <span className="tab-name">{name}</span>
                    {typeLabel.toLocaleLowerCase() !== name.trim().toLocaleLowerCase() ? (
                      <span className="tab-type">{typeLabel}</span>
                    ) : null}
                  </button>
                )
              })}
            </nav>
          </div>

          <div className="library-action-cluster">
            <form className="library-inline-search" onSubmit={submitSearch}>
              <Search aria-hidden="true" className="search-icon" size={15} />
              <label className="sr-only" htmlFor="library-query">搜索 Emby 媒体</label>
              <input
                autoComplete="off"
                id="library-query"
                maxLength={120}
                name="library-query"
                onChange={(event) => setQueryText(event.target.value)}
                placeholder="搜索…"
                type="search"
                value={queryText}
              />
              {queryText ? (
                <button aria-label="清空搜索内容" className="search-clear-btn" onClick={clearSearch} type="button">
                  <X aria-hidden="true" size={14} />
                </button>
              ) : null}
              {queryText.trim() ? (
                <button className="search-submit-btn" type="submit">搜</button>
              ) : null}
            </form>

            <div className="library-meta-actions">
              <span className="library-count-tag">{total} 部</span>
              {selectedLibrary && !submittedQuery ? (
                <button
                  className="library-refresh-btn"
                  disabled={refreshLibrary.isPending}
                  onClick={() => refreshLibrary.mutate(selectedLibrary.id)}
                  title="刷新当前媒体库元数据"
                  type="button"
                >
                  <RefreshCw className={refreshLibrary.isPending ? 'spin' : ''} size={14} />
                  <span>{refreshLibrary.isPending ? '刷新中' : '刷新'}</span>
                </button>
              ) : null}
              {!submittedQuery && total > pageSize ? (
                <div className="library-mini-pagination">
                  <IconButton
                    disabled={page === 0}
                    label="上一页"
                    onClick={() => changePage(Math.max(0, page - 1))}
                    subtle
                  >
                    <ChevronLeft size={16} />
                  </IconButton>
                  <span className="page-indicator">
                    {page + 1}/{Math.max(1, Math.ceil(total / pageSize))}
                  </span>
                  <IconButton
                    disabled={(page + 1) * pageSize >= total}
                    label="下一页"
                    onClick={() => changePage(page + 1)}
                    subtle
                  >
                    <ChevronRight size={16} />
                  </IconButton>
                </div>
              ) : null}
            </div>
          </div>
        </div>

        {submittedQuery ? (
          <div className="search-active-pill">
            <span>正在显示“<strong>{submittedQuery}</strong>”的搜索结果（共 {total} 部）</span>
            <button className="search-active-clear" onClick={clearSearch} type="button">
              <X size={14} />
              清除搜索
            </button>
          </div>
        ) : null}

        {!submittedQuery && page === 0 && activeSpotlight ? (
          <div className="library-hero-spotlight">
            <div
              className="spotlight-backdrop"
              key={activeSpotlight.id}
              style={{ backgroundImage: `url(${embyPrimaryImageURL(activeSpotlight.id)})` }}
            />
            <div className="spotlight-vignette" />
            <div className="spotlight-content">
              <div className="spotlight-poster-wrap">
                <img
                  alt=""
                  className="spotlight-poster"
                  decoding="async"
                  height={150}
                  key={activeSpotlight.id}
                  src={embyPrimaryImageURL(activeSpotlight.id)}
                  width={100}
                />
              </div>
              <div className="spotlight-body">
                <div className="spotlight-badges">
                  <span className="spotlight-tag">
                    {(activeSpotlight.playbackPositionMs ?? 0) >= 30_000 && !activeSpotlight.played
                      ? '继续观看'
                      : activeSpotlight.played
                        ? '重温推荐'
                        : selectedLibrary
                          ? selectedLibrary.name
                          : '精选推荐'}
                  </span>
                  <span className="spotlight-tag">{mediaTypeLabel(activeSpotlight.type)}</span>
                  {activeSpotlight.played ? (
                    <span className="spotlight-status played">已看完</span>
                  ) : (activeSpotlight.playbackPositionMs ?? 0) >= 30_000 ? (
                    <span className="spotlight-status in-progress">
                      {playbackStatus(activeSpotlight)}
                    </span>
                  ) : null}
                  {activeSpotlight.year ? <span className="spotlight-year">{activeSpotlight.year}</span> : null}
                </div>
                <h2 className="spotlight-title">{activeSpotlight.name}</h2>
                <div className="spotlight-actions">
                  <button
                    className="spotlight-play-btn"
                    onClick={() => selectItem(activeSpotlight.id)}
                    type="button"
                  >
                    <Play fill="currentColor" size={14} />
                    {(activeSpotlight.playbackPositionMs ?? 0) >= 30_000 && !activeSpotlight.played
                      ? '继续播放'
                      : activeSpotlight.played
                        ? '重新播放'
                        : '立即播放'}
                  </button>
                  <button
                    className="spotlight-detail-btn"
                    onClick={() => selectItem(activeSpotlight.id)}
                    type="button"
                  >
                    影片详情
                  </button>
                </div>
              </div>
            </div>

            {spotlightCandidates.length > 1 ? (
              <div className="spotlight-carousel-controls">
                <div className="spotlight-dots">
                  {spotlightCandidates.map((c, i) => (
                    <button
                      aria-label={`切换到推荐 ${i + 1}`}
                      className={i === spotlightIndex ? 'spotlight-dot active' : 'spotlight-dot'}
                      key={c.id}
                      onClick={() => setSpotlightIndex(i)}
                      type="button"
                    />
                  ))}
                </div>
              </div>
            ) : null}
          </div>
        ) : null}

        {!submittedQuery && page === 0 && continueWatchingItems.length > 0 ? (
          <div className="continue-watching-section">
            <div className="continue-watching-header">
              <div className="continue-watching-title">
                <span aria-hidden="true" className="live-indicator" />
                <strong>继续观看</strong>
                <span className="library-count-tag">{continueWatchingItems.length}</span>
              </div>
            </div>
            <div className="continue-watching-rail">
              {continueWatchingItems.map((item) => {
                const status = playbackStatus(item)
                return (
                  <button
                    className="continue-card"
                    key={item.id}
                    onClick={() => selectItem(item.id)}
                    type="button"
                  >
                    <span className="continue-card-poster">
                      <img
                        alt=""
                        decoding="async"
                        loading="lazy"
                        src={embyPrimaryImageURL(item.id)}
                      />
                      <span aria-hidden="true" className="continue-card-overlay">
                        <span className="continue-play-circle">
                          <Play fill="currentColor" size={15} />
                        </span>
                      </span>
                      <span aria-hidden="true" className="continue-progress-bar">
                        <span
                          className="continue-progress-fill"
                          style={{
                            width: `${Math.min(96, Math.max(12, ((item.playbackPositionMs ?? 0) / 7200000) * 100))}%`,
                          }}
                        />
                      </span>
                    </span>
                    <span className="continue-card-meta">
                      <strong className="continue-name" title={item.name}>{item.name}</strong>
                      <small className="continue-sub">{status}</small>
                    </span>
                  </button>
                )
              })}
            </div>
          </div>
        ) : null}

        <div className="library-filter-bar">
          <div className="filter-group">
            <button
              className={typeFilter === 'all' ? 'filter-pill active' : 'filter-pill'}
              onClick={() => setTypeFilter('all')}
              type="button"
            >
              全部类型
            </button>
            <button
              className={typeFilter === 'Movie' ? 'filter-pill active' : 'filter-pill'}
              onClick={() => setTypeFilter('Movie')}
              type="button"
            >
              电影
            </button>
            <button
              className={typeFilter === 'Series' ? 'filter-pill active' : 'filter-pill'}
              onClick={() => setTypeFilter('Series')}
              type="button"
            >
              剧集
            </button>
            <span aria-hidden="true" className="filter-divider" />
            <button
              className={statusFilter === 'all' ? 'filter-pill active' : 'filter-pill'}
              onClick={() => setStatusFilter('all')}
              type="button"
            >
              全部状态
            </button>
            <button
              className={statusFilter === 'in-progress' ? 'filter-pill active' : 'filter-pill'}
              onClick={() => setStatusFilter('in-progress')}
              type="button"
            >
              在看中
            </button>
            <button
              className={statusFilter === 'unplayed' ? 'filter-pill active' : 'filter-pill'}
              onClick={() => setStatusFilter('unplayed')}
              type="button"
            >
              未看
            </button>
            <button
              className={statusFilter === 'played' ? 'filter-pill active' : 'filter-pill'}
              onClick={() => setStatusFilter('played')}
              type="button"
            >
              已看
            </button>
          </div>

          <div className="sort-select-wrap">
            <ArrowDownUp aria-hidden="true" size={13} />
            <button
              className="sort-select-btn"
              onClick={() => {
                const cycle: Array<'default' | 'year-desc' | 'year-asc' | 'name-asc'> = [
                  'default',
                  'year-desc',
                  'year-asc',
                  'name-asc',
                ]
                const next = cycle[(cycle.indexOf(sortBy) + 1) % cycle.length]
                setSortBy(next)
              }}
              type="button"
            >
              {sortBy === 'default'
                ? '排序：默认推荐'
                : sortBy === 'year-desc'
                  ? '排序：最新年份'
                  : sortBy === 'year-asc'
                    ? '排序：经典上映'
                    : '排序：名称 A-Z'}
            </button>
            {hasActiveFilters ? (
              <button className="filter-reset-btn" onClick={resetFilters} title="重置所有筛选" type="button">
                重置筛选
              </button>
            ) : null}
          </div>
        </div>

        <div className="library-results">
          {result.isLoading && items.length === 0 ? (
            <div aria-label="正在加载媒体海报" className="library-poster-grid" role="status">
              {Array.from({ length: 12 }).map((_, i) => (
                <div className="library-poster-skeleton" key={i}>
                  <div className="skeleton-poster-frame" />
                  <div className="skeleton-poster-title" />
                </div>
              ))}
            </div>
          ) : null}
          {result.isError ? <div className="inline-error"><CircleAlert size={18} /><div><strong>媒体内容读取失败</strong><span>{result.error.message}</span></div><button onClick={() => void result.refetch()} type="button">重试</button></div> : null}
          {!result.isLoading && !result.isError && items.length === 0 ? (
            <div className="empty-state">
              <BookOpen size={28} />
              <span>{hasActiveFilters ? '未找到符合筛选条件的媒体内容' : submittedQuery ? '没有匹配的媒体' : '此媒体库暂无可浏览内容'}</span>
              {hasActiveFilters ? <button className="secondary-command" onClick={resetFilters} type="button">清除筛选条件</button> : null}
            </div>
          ) : null}
          <div className="library-poster-grid">
            {items.map((item) => <LibraryPosterCard item={item} key={item.id} onSelect={selectItem} selected={item.id === itemId} />)}
          </div>
          {!submittedQuery && total > pageSize ? (
            <div aria-label="底部页面导航" className="library-pagination library-pagination-bottom">
              <IconButton disabled={page === 0} label="上一页" onClick={() => changePage(Math.max(0, page - 1))} subtle><ChevronLeft size={17} /></IconButton>
              <span>{page + 1} / {Math.max(1, Math.ceil(total / pageSize))}</span>
              <IconButton disabled={(page + 1) * pageSize >= total} label="下一页" onClick={() => changePage(page + 1)} subtle><ChevronRight size={17} /></IconButton>
            </div>
          ) : null}
        </div>
      </div>

      {itemId ? (
        <aside className="library-detail" aria-label="媒体详情">
          <button className="library-detail-back" onClick={closeItem} type="button">
            <ChevronLeft aria-hidden="true" size={18} />
            返回媒体列表
          </button>
          {detail.isLoading && !detail.data ? <div className="status-loading">正在读取媒体详情…</div> : null}
          {detail.isError ? <div className="inline-error"><CircleAlert size={18} /><div><strong>详情读取失败</strong><span>{detail.error.message}</span></div><button onClick={() => void detail.refetch()} type="button">重试</button></div> : null}
          {detail.data ? <LibraryItemDetail inPagePlayback={inPagePlayback} item={detail.data} onDeleted={closeItem} onPlay={startPlay} onRefresh={(id) => refreshItem.mutate(id)} refreshing={refreshItem.isPending} /> : null}
        </aside>
      ) : null}
      {inPagePlayback && playTarget ? (
        <LibraryPlayer
          externalUrl={playTarget.externalUrl}
          itemId={playTarget.id}
          nextEpisodeLabel={nextEpisode ? episodeLabel(nextEpisode, detail.data?.name ?? '') : undefined}
          onClose={closePlay}
          onNextEpisode={nextEpisode ? () => startPlay(nextEpisode) : undefined}
          title={playTarget.title}
        />
      ) : null}
    </section>
  )
}

function LibraryPosterCard({ item, selected, onSelect }: { item: EmbyItem; selected: boolean; onSelect: (id: string) => void }) {
  const [failed, setFailed] = useState(false)
  const isPlayed = Boolean(item.played)
  const isProgress = (item.playbackPositionMs ?? 0) >= 30_000 && !isPlayed
  const status = isPlayed || isProgress ? playbackStatus(item) : ''

  return (
    <button aria-pressed={selected} className={selected ? 'library-poster-card selected' : 'library-poster-card'} onClick={() => onSelect(item.id)} type="button">
      <span className="library-poster-frame">
        {!failed ? (
          <img
            alt=""
            decoding="async"
            height={240}
            loading="lazy"
            onError={() => setFailed(true)}
            src={embyPrimaryImageURL(item.id)}
            width={160}
          />
        ) : (
          <span aria-hidden="true" className="library-poster-fallback"><Film size={28} /></span>
        )}
        <div aria-hidden="true" className="poster-vignette" />
        <div aria-hidden="true" className="poster-hover-overlay">
          <span className="poster-play-icon"><Play fill="currentColor" size={18} /></span>
        </div>
        {status ? (
          <span className={isPlayed ? 'library-poster-badge played' : 'library-poster-badge in-progress'}>
            {status}
          </span>
        ) : null}
      </span>
      <span className="library-poster-meta">
        <strong className="poster-title">{item.name}</strong>
        <span className="poster-subtitle">
          <span className="poster-type">{mediaTypeLabel(item.type)}</span>
          {item.year ? <span className="poster-year">· {item.year}</span> : null}
        </span>
      </span>
    </button>
  )
}

function LibraryItemDetail({ item, inPagePlayback, onDeleted, onPlay, onRefresh, refreshing }: {
  item: EmbyItemDetail
  inPagePlayback: boolean
  onDeleted: () => void
  onPlay: (target: Pick<EmbyEpisode, 'id' | 'name' | 'externalUrl'>) => void
  onRefresh: (id: string) => void
  refreshing: boolean
}) {
  const queryClient = useQueryClient()
  const originalTitle = visibleOriginalTitle(item)
  const genres = localizedGenres(item.genres ?? [])
  const [posterFailed, setPosterFailed] = useState(false)
  const [subtitleResults, setSubtitleResults] = useState<EmbyRemoteSubtitle[] | null>(null)
  const previewDelete = useMutation({ mutationFn: () => previewEmbyItemDelete(item.id) })
  const confirmDelete = useMutation({
    mutationFn: () => deleteEmbyItem(item.id),
    onSuccess: async () => {
      onDeleted()
      queryClient.removeQueries({ queryKey: ['emby-item', item.id] })
      await queryClient.invalidateQueries({ queryKey: ['emby-library-items'] })
      await queryClient.invalidateQueries({ queryKey: ['emby-search'] })
    },
  })
  const searchSubtitles = useMutation({
    mutationFn: () => searchEmbyRemoteSubtitles(item.id, 'chi'),
    onSuccess: (data) => setSubtitleResults(data.items),
  })
  const downloadSubtitle = useMutation({
    mutationFn: (subtitleId: string) => downloadEmbyRemoteSubtitle(item.id, subtitleId),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['emby-local-subtitle', item.id] })
    },
  })
  const localSubtitle = useQuery({
    queryKey: ['emby-local-subtitle', item.id],
    queryFn: () => fetchLocalSubtitle(item.id),
    enabled: item.type !== 'Series',
    retry: false,
    staleTime: 60_000,
  })
  const preview = previewDelete.data
  const busy = previewDelete.isPending || confirmDelete.isPending || refreshing || searchSubtitles.isPending || downloadSubtitle.isPending
  const deleteError = previewDelete.error ?? confirmDelete.error
  const subtitleError = searchSubtitles.error ?? downloadSubtitle.error
  const canSearchSubtitles = item.type !== 'Series'

  return <>
    <div
      aria-hidden="true"
      className="detail-ambient-backdrop"
      style={{ backgroundImage: `url(${embyPrimaryImageURL(item.id)})` }}
    />
    <div className="library-detail-hero">
      <div className="library-detail-poster">
        {!posterFailed ? (
          <>
            <div aria-hidden="true" className="detail-poster-backdrop" style={{ backgroundImage: `url(${embyPrimaryImageURL(item.id)})` }} />
            <div aria-hidden="true" className="detail-poster-vignette" />
            <img alt="" className="detail-poster-foreground" decoding="async" height={360} onError={() => setPosterFailed(true)} src={embyPrimaryImageURL(item.id)} width={240} />
          </>
        ) : (
          <span aria-hidden="true" className="library-poster-fallback"><Film size={36} /></span>
        )}
      </div>
      <div className="library-detail-hero-copy">
        <div className="library-detail-heading">
          <div>
            <h2>{item.name}</h2>
            {originalTitle ? <span className="original-title">{originalTitle}</span> : null}
          </div>
          <span className="media-type-chip">{mediaTypeLabel(item.type)}</span>
        </div>
        <dl className="library-facts compact">
          <div><dt>年份</dt><dd>{item.year || '未知'}</dd></div>
          <div><dt>时长</dt><dd>{item.runtimeMinutes ? `${item.runtimeMinutes} 分钟` : '未提供'}</dd></div>
          <div>
            <dt>评分</dt>
            <dd className="rating-cell">
              {item.communityRating ? (
                <>
                  <Star className="rating-star" size={14} />
                  <span>{item.communityRating.toFixed(1)}</span>
                </>
              ) : (
                '未提供'
              )}
            </dd>
          </div>
          <div><dt>进度</dt><dd>{playbackStatus(item)}</dd></div>
          {item.type === 'Series' ? null : (
            <div>
              <dt>字幕</dt>
              <dd>{localSubtitle.isLoading ? '检查中' : localSubtitle.data ? '已挂中文' : '未挂中文'}</dd>
            </div>
          )}
        </dl>
        {genres.length ? <div className="library-genres">{genres.map((genre) => <span key={genre}>{genre}</span>)}</div> : null}
        {item.type === 'Series' ? null : (
          <div className="library-detail-actions library-detail-actions-primary">
            <LibraryWatchAction
              busy={busy}
              externalUrl={item.externalUrl}
              inPage={inPagePlayback}
              item={item}
              name={item.name}
              onPlay={() => onPlay(item)}
            />
          </div>
        )}
      </div>
    </div>
    <p className="library-overview">{item.overview || '暂未提供简介。'}</p>
    <details className="library-more">
      <summary>更多信息</summary>
      <dl>
        {originalTitle ? <div><dt>原名</dt><dd>{originalTitle}</dd></div> : null}
        <div>
          <dt>TMDB 编号</dt>
          <dd>
            {item.providerIds?.Tmdb ? (
              <a
                className="tmdb-link"
                href={`https://www.themoviedb.org/${item.type === 'Series' ? 'tv' : 'movie'}/${item.providerIds.Tmdb}`}
                rel="noopener noreferrer"
                target="_blank"
                title="在 The Movie Database 查看"
              >
                <span>{item.providerIds.Tmdb}</span>
                <ExternalLink size={12} />
              </a>
            ) : (
              '未绑定'
            )}
          </dd>
        </div>
        <div><dt>媒体源</dt><dd>{item.type === 'Series' ? '由分集提供' : `${item.mediaSourceCount} 个`}</dd></div>
      </dl>
    </details>
    <div className="library-detail-actions library-detail-actions-secondary">
      <button className="secondary-command" disabled={busy} onClick={() => onRefresh(item.id)} type="button"><RefreshCw size={16} />{refreshing ? '已提交…' : '刷新元数据'}</button>
      {canSearchSubtitles ? (
        <button
          className="secondary-command"
          disabled={busy}
          onClick={() => {
            setSubtitleResults(null)
            searchSubtitles.mutate()
          }}
          type="button"
        >
          <Captions size={16} />
          {searchSubtitles.isPending ? '正在搜索…' : '搜中文字幕'}
        </button>
      ) : (
        <span className="library-subtitle-hint">剧集请在 Emby 分集条目上搜索字幕</span>
      )}
      {preview ? null : <button className="danger-button" disabled={busy} onClick={() => previewDelete.mutate()} type="button"><Trash2 size={16} />{previewDelete.isPending ? '正在读取删除预览…' : '从 Emby 删除'}</button>}
    </div>
    {item.type === 'Series' ? <LibraryEpisodes inPagePlayback={inPagePlayback} onPlay={onPlay} seriesId={item.id} seriesTitle={item.name} /> : null}
    {subtitleResults ? (
      <section className="library-subtitle-panel" aria-label="中文字幕搜索结果">
        <div className="library-subtitle-heading">
          <strong>中文字幕</strong>
          <IconButton label="关闭字幕结果" onClick={() => { setSubtitleResults(null); searchSubtitles.reset(); downloadSubtitle.reset() }} subtle><X size={16} /></IconButton>
        </div>
        {downloadSubtitle.isSuccess ? <p className="library-subtitle-status" role="status">字幕已保存到影片目录。请用更新后的 App 重新打开本集，即可在字幕列表里选择。</p> : null}
        {subtitleError ? <div className="source-warning error" role="alert"><CircleAlert size={16} /><span>{subtitleError.message}</span></div> : null}
        {subtitleResults.length === 0 ? <p className="library-subtitle-status">未找到可用中文字幕。可在设置中填写 Assrt Token；未配置时会回退到 Emby 字幕插件。</p> : (
          <ul className="library-subtitle-list">
            {subtitleResults.map((subtitle) => {
              const meta = [
                subtitle.format?.toUpperCase(),
                subtitle.providerName,
                subtitle.isHashMatch ? '精确匹配' : null,
                subtitle.downloadCount ? `${subtitle.downloadCount} 次下载` : null,
              ].filter(Boolean).join(' · ')
              const downloading = downloadSubtitle.isPending && downloadSubtitle.variables === subtitle.id
              return (
                <li key={subtitle.id}>
                  <div>
                    <strong>{subtitle.name}</strong>
                    {meta ? <small>{meta}</small> : null}
                  </div>
                  <button className="secondary-command" disabled={busy} onClick={() => downloadSubtitle.mutate(subtitle.id)} type="button">
                    {downloading ? '下载中…' : '下载'}
                  </button>
                </li>
              )
            })}
          </ul>
        )}
      </section>
    ) : subtitleError ? <div className="source-warning error" role="alert"><CircleAlert size={16} /><span>{subtitleError.message}</span></div> : null}
    {preview ? (
      <div className="library-delete-confirm">
        <p>{deletePreviewCopy(preview)}</p>
        {deleteError ? <div className="source-warning error" role="alert"><CircleAlert size={16} /><span>{deleteError.message}</span></div> : null}
        <div className="library-detail-actions">
          <button className="danger-button" disabled={confirmDelete.isPending} onClick={() => confirmDelete.mutate()} type="button">{confirmDelete.isPending ? '正在删除' : '确认删除'}</button>
          <button className="secondary-command" disabled={confirmDelete.isPending} onClick={() => previewDelete.reset()} type="button">取消</button>
        </div>
      </div>
    ) : deleteError ? <div className="source-warning error" role="alert"><CircleAlert size={16} /><span>{deleteError.message}</span></div> : null}
  </>
}

function deletePreviewCopy(preview: EmbyDeletePreview) {
  const series = preview.type === 'Series' ? '及全部分集' : ''
  const versions = preview.versionCount > 1 ? `的 ${preview.versionCount} 个版本` : ''
  if (preview.cloudKept) {
    return `将从 Emby 删除「${preview.name}」${versions}${series}。NAS 上约 ${preview.fileCount} 个 STRM 和同名字幕会被清掉，115 网盘文件不会删除。`
  }
  return `将从 Emby 删除「${preview.name}」${versions}${series}。NAS 上约 ${preview.fileCount} 个本地媒体文件会被删除，此操作不可恢复。`
}

function libraryCollectionLabel(type?: string) {
  if (type === 'movies') return '电影'
  if (type === 'tvshows') return '剧集'
  return '媒体库'
}

function mediaTypeLabel(type: string) {
  if (type === 'Movie') return '电影'
  if (type === 'Series') return '剧集'
  return '媒体'
}

function visibleOriginalTitle(item: EmbyItemDetail) {
  const originalTitle = item.originalTitle?.trim()
  if (!originalTitle || normalizeTitle(originalTitle) === normalizeTitle(item.name)) return null
  return originalTitle
}

function normalizeTitle(value: string) {
  return value.normalize('NFKC').trim().toLocaleLowerCase()
}

const genreLabels: Record<string, string> = {
  action: '动作',
  'action & adventure': '动作冒险',
  adventure: '冒险',
  animation: '动画',
  biography: '传记',
  comedy: '喜剧',
  crime: '犯罪',
  documentary: '纪录片',
  drama: '剧情',
  family: '家庭',
  fantasy: '奇幻',
  'film-noir': '黑色电影',
  history: '历史',
  horror: '恐怖',
  kids: '儿童',
  music: '音乐',
  musical: '音乐剧',
  mystery: '悬疑',
  news: '新闻',
  reality: '真人秀',
  romance: '爱情',
  'sci-fi & fantasy': '科幻奇幻',
  'science fiction': '科幻',
  short: '短片',
  soap: '肥皂剧',
  sport: '体育',
  talk: '脱口秀',
  thriller: '惊悚',
  'tv movie': '电视电影',
  war: '战争',
  'war & politics': '战争政治',
  western: '西部',
}

function localizedGenres(genres: string[]) {
  return [...new Set(genres.map((genre) => {
    const value = genre.trim()
    return genreLabels[value.toLocaleLowerCase()] ?? value
  }).filter(Boolean))]
}
