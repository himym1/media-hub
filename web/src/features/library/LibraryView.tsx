import { useEffect, useMemo, useRef, useState, type FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  ArrowDownUp,
  BookOpen,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  ChevronUp,
  CircleAlert,
  Clapperboard,
  Film,
  LayoutGrid,
  Music,
  Play,
  RefreshCw,
  Search,
  Tv,
  X,
} from 'lucide-react'
import {
  embyPrimaryImageURL,
  getEmbyEpisodes,
  getEmbyItem,
  getEmbyLibraries,
  getEmbyLibraryItems,
  getSystemOverview,
  refreshEmbyItem,
  refreshEmbyLibrary,
  searchEmbyItems,
  type EmbyItem,
} from '../../shared/api/mediaHub'
import { canPlayNatively } from '../../shared/desktop/nativePlayback'
import { commitUrl } from '../../shared/navigation/urlState'
import { IconButton } from '../../shared/ui/IconButton'
import { LibraryItemDetail } from './LibraryItemDetail'
import { LibraryDetailSkeleton, LibraryPosterGridSkeleton } from './LibrarySkeletons'
import { LibraryPlayer } from './LibraryPlayer'
import { episodeLabel, mediaTypeLabel, playbackStatus } from './libraryPlayback'
import { adultLibraryId, childLibraries, isSharedEmbyId, libraryDisplayName, libraryRootId, mineLibraries, rootLibraries, sharedLibraries } from './libraryGroups'
import { ensureQueueContains, episodeQueue, playableLibraryQueue } from './libraryPlaylist'
import { libraryBrowseSort, librarySortLabel, nextLibrarySort, type LibrarySort } from './librarySort'

const pageSize = 24
const playQueueLimit = 100 // Emby 浏览上限；分组播放列表一次最多装这么多

type LibraryScope = 'mine' | 'shared'

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
  const [sortBy, setSortBy] = useState<LibrarySort>('default')
  const [typeFilter, setTypeFilter] = useState<'all' | 'Movie' | 'Series'>('all')
  const [statusFilter, setStatusFilter] = useState<'all' | 'in-progress' | 'unplayed' | 'played'>('all')
  const [spotlightIndex, setSpotlightIndex] = useState(0)
  const [isExpanded, setIsExpanded] = useState(false)
  const [scope, setScope] = useState<LibraryScope>(() => (isSharedEmbyId(locationValue('library')) ? 'shared' : 'mine'))
  const tabsRef = useRef<HTMLDivElement>(null)

  const scrollTabs = (direction: 'left' | 'right') => {
    if (!tabsRef.current) return
    const scrollAmount = 260
    tabsRef.current.scrollBy({
      left: direction === 'left' ? -scrollAmount : scrollAmount,
      behavior: 'smooth',
    })
  }

  const handleTabsWheel = (e: React.WheelEvent) => {
    if (e.deltaY !== 0 && tabsRef.current) {
      tabsRef.current.scrollLeft += e.deltaY
    }
  }

  const libraries = useQuery({ queryKey: ['emby-libraries'], queryFn: getEmbyLibraries })
  const overview = useQuery({ queryKey: ['system-overview'], queryFn: getSystemOverview, retry: false })
  const allLibraries = useMemo(() => libraries.data?.libraries ?? [], [libraries.data?.libraries])
  const localLibraries = useMemo(() => mineLibraries(allLibraries), [allLibraries])
  const remoteLibraries = useMemo(() => sharedLibraries(allLibraries), [allLibraries])
  const hasShared = remoteLibraries.length > 0
  const sharedHealth = overview.data?.integrations.find((integration) => integration.id === 'shared-emby')
  const sharedUnavailable = Boolean(
    sharedHealth
    && sharedHealth.status !== 'unconfigured'
    && sharedHealth.status !== 'healthy'
    && !hasShared,
  )
  const scopeLibraries = scope === 'shared' ? remoteLibraries : localLibraries
  const topLibraries = useMemo(() => rootLibraries(scopeLibraries), [scopeLibraries])
  const adultGroups = useMemo(() => childLibraries(scopeLibraries, adultLibraryId), [scopeLibraries])
  const selectedRootId = libraryRootId(scopeLibraries, libraryId)
  const adultBrowsing = selectedRootId === adultLibraryId
  const browseSort = libraryBrowseSort(sortBy, adultBrowsing)
  const libraryItems = useQuery({
    queryKey: ['emby-library-items', libraryId, page, browseSort],
    queryFn: () => getEmbyLibraryItems(libraryId!, page * pageSize, pageSize, browseSort),
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
  const inPagePlayback = canPlayNatively()
  const libraryPlayQueue = useQuery({
    queryKey: ['emby-library-play-queue', libraryId, browseSort],
    queryFn: () => getEmbyLibraryItems(libraryId!, 0, playQueueLimit, browseSort),
    enabled: Boolean(inPagePlayback && playId && libraryId && !submittedQuery && detail.data?.type !== 'Series'),
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

  useEffect(() => {
    const restoreLocation = () => {
      const nextLibrary = locationValue('library')
      setLibraryId(nextLibrary)
      setItemId(locationValue('media'))
      setPlayId(locationValue('play'))
      setPage(0)
      setQueryText('')
      setSubmittedQuery('')
      setScope(isSharedEmbyId(nextLibrary) ? 'shared' : 'mine')
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
    if (!hasShared && scope === 'shared') {
      setScope('mine')
      return
    }
    if (libraryId && scopeLibraries.some((library) => library.id === libraryId)) return
    const fallback = (scope === 'shared' ? remoteLibraries : rootLibraries(localLibraries))[0]?.id ?? null
    setLibraryId(fallback)
    setItemId(null)
    setPlayId(null)
    setPage(0)
    commitUrl({ library: fallback, media: null, play: null }, 'replace')
  }, [libraries.data, libraryId, scope, scopeLibraries, hasShared, localLibraries, remoteLibraries])

  const selectScope = (next: LibraryScope) => {
    if (next === scope) return
    setScope(next)
    setSubmittedQuery('')
    setQueryText('')
    setItemId(null)
    setPlayId(null)
    setPage(0)
    setSpotlightIndex(0)
    const fallback = (next === 'shared' ? remoteLibraries : rootLibraries(localLibraries))[0]?.id ?? null
    setLibraryId(fallback)
    commitUrl({ library: fallback, media: null, play: null })
  }
  const selectLibrary = (id: string) => {
    setLibraryId(id)
    setItemId(null)
    setPlayId(null)
    setPage(0)
    setQueryText('')
    setSubmittedQuery('')
    setSpotlightIndex(0)
    setScope(isSharedEmbyId(id) ? 'shared' : 'mine')
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
  const startPlay = (target: { id: string }) => {
    if (!inPagePlayback) return
    setPlayId(target.id)
    commitUrl({ play: target.id })
  }
  const startQueueItem = (id: string) => {
    if (!inPagePlayback || id === playId) return
    const episode = episodes.data?.items.find((item) => item.id === id)
    if (episode) {
      startPlay(episode)
      return
    }
    setItemId(id)
    setPlayId(id)
    commitUrl({ media: id, play: id })
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
  const selectPerson = (name: string) => {
    const trimmed = name.trim()
    if (!trimmed) return
    setQueryText(trimmed)
    setSubmittedQuery(trimmed)
    setItemId(null)
    setPlayId(null)
    setPage(0)
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
  const rawItems = useMemo(() => {
    const list = result.data?.items ?? []
    if (!submittedQuery) return list
    return list.filter((item) => (scope === 'shared' ? isSharedEmbyId(item.id) : !isSharedEmbyId(item.id)))
  }, [result.data?.items, submittedQuery, scope])

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
      list = list.filter((item) => {
        if (typeFilter === 'Movie' && adultBrowsing) return item.type === 'Movie' || item.type === 'Video'
        return item.type === typeFilter
      })
    }
    if (statusFilter === 'in-progress') {
      list = list.filter((item) => (item.playbackPositionMs ?? 0) >= 30_000 && !item.played)
    } else if (statusFilter === 'played') {
      list = list.filter((item) => Boolean(item.played))
    } else if (statusFilter === 'unplayed') {
      list = list.filter((item) => !item.played && (item.playbackPositionMs ?? 0) < 30_000)
    }

    return list
  }, [rawItems, typeFilter, statusFilter, adultBrowsing])

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
  const playingId = playEpisode?.id
    ?? (playId && detail.data && detail.data.type !== 'Series' && detail.data.id === playId ? detail.data.id : playId)
  const playingTitle = playEpisode
    ? episodeLabel(playEpisode, detail.data?.name ?? '')
    : playId && detail.data && detail.data.type !== 'Series' && detail.data.id === playId
      ? detail.data.name
      : items.find((item) => item.id === playId)?.name ?? (playId ? '播放' : '')
  const playingUrl = playEpisode?.externalUrl
    ?? (playId && detail.data && detail.data.type !== 'Series' && detail.data.id === playId ? detail.data.externalUrl : '')
  const playerQueue = useMemo(() => {
    if (playEpisode && episodes.data?.items.length) {
      return episodeQueue(episodes.data.items, detail.data?.name ?? '')
    }
    const source = submittedQuery ? (search.data?.items ?? []) : (libraryPlayQueue.data?.items ?? rawItems)
    return ensureQueueContains(
      playableLibraryQueue(source),
      playingId ? { id: playingId, title: playingTitle } : null,
    )
  }, [detail.data?.name, episodes.data?.items, libraryPlayQueue.data?.items, playEpisode, playingId, playingTitle, rawItems, search.data?.items, submittedQuery])
  const playTarget = playingId
    ? {
      id: playingId,
      title: playerQueue.find((item) => item.id === playingId)?.title ?? playingTitle,
      externalUrl: playingUrl,
    }
    : null

  return (
    <section className={itemId ? 'library-page has-detail' : 'library-page'}>
      {libraries.isError ? <div className="inline-error"><CircleAlert size={18} /><div><strong>媒体库读取失败</strong><span>{libraries.error.message}</span></div><button onClick={() => void libraries.refetch()} type="button">重试</button></div> : null}
      {sharedUnavailable ? (
        <div className="source-warning error" role="alert">
          <CircleAlert size={16} />
          <span>共享 Emby 暂时不可用：{sharedHealth?.detail || '无法读取共享库'}。我的库仍可正常使用。</span>
        </div>
      ) : null}
      {mutationError ? <div className="source-warning error" role="alert"><CircleAlert size={16} /><span>{mutationError.message}</span></div> : null}

      <div className="library-browse">
        <h1 className="sr-only">媒体库</h1>
        <div className="library-header-bar">
          <div className="library-nav-cluster">
            {hasShared ? (
              <div aria-label="媒体库来源" className="library-scope-switch" role="tablist">
                <button
                  aria-selected={scope === 'mine'}
                  className={scope === 'mine' ? 'scope-switch-btn active' : 'scope-switch-btn'}
                  onClick={() => selectScope('mine')}
                  role="tab"
                  type="button"
                >
                  我的库
                </button>
                <button
                  aria-selected={scope === 'shared'}
                  className={scope === 'shared' ? 'scope-switch-btn active' : 'scope-switch-btn'}
                  onClick={() => selectScope('shared')}
                  role="tab"
                  type="button"
                >
                  共享库
                </button>
              </div>
            ) : null}

            {topLibraries.length > 3 ? (
              <button
                aria-label="向左滚动媒体库"
                className="library-nav-scroll-btn left"
                onClick={() => scrollTabs('left')}
                title="向左滚动"
                type="button"
              >
                <ChevronLeft size={16} />
              </button>
            ) : null}

            <nav
              aria-label={scope === 'shared' ? '共享 Emby 媒体库' : '我的 Emby 媒体库'}
              className="library-tabs"
              onWheel={handleTabsWheel}
              ref={tabsRef}
            >
              {topLibraries.map((library) => {
                const typeLabel = libraryCollectionLabel(library.collectionType)
                const name = libraryDisplayName(library.name)
                const isSelected = library.id === selectedRootId
                const Icon = getLibraryIcon(name, library.collectionType)
                return (
                  <button
                    aria-pressed={isSelected}
                    className={isSelected ? 'library-tab-pill active' : 'library-tab-pill'}
                    key={library.id}
                    onClick={() => selectLibrary(library.id)}
                    type="button"
                  >
                    <Icon className="tab-icon" size={14} />
                    <span className="tab-name">{name}</span>
                    {typeLabel.toLocaleLowerCase() !== name.trim().toLocaleLowerCase() ? (
                      <span className="tab-type">{typeLabel}</span>
                    ) : null}
                  </button>
                )
              })}
            </nav>

            {topLibraries.length > 3 ? (
              <button
                aria-label="向右滚动媒体库"
                className="library-nav-scroll-btn right"
                onClick={() => scrollTabs('right')}
                title="向右滚动"
                type="button"
              >
                <ChevronRight size={16} />
              </button>
            ) : null}

            {allLibraries.length > 0 ? (
              <button
                aria-expanded={isExpanded}
                className={isExpanded ? 'library-expand-toggle-btn active' : 'library-expand-toggle-btn'}
                onClick={() => setIsExpanded((prev) => !prev)}
                title={isExpanded ? '收起全部媒体库' : '展开查看全部媒体库'}
                type="button"
              >
                <LayoutGrid size={14} />
                <span>全部 ({topLibraries.length})</span>
                {isExpanded ? <ChevronUp size={13} /> : <ChevronDown size={13} />}
              </button>
            ) : null}
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
                placeholder={scope === 'shared' ? '搜索共享库…' : '搜索…'}
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
              {selectedLibrary && !submittedQuery && !isSharedEmbyId(selectedLibrary.id) ? (
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

        {selectedRootId === adultLibraryId && adultGroups.length > 0 ? (
          <nav aria-label="成人影视分组" className="library-group-chips">
            <button
              aria-pressed={libraryId === adultLibraryId}
              className={libraryId === adultLibraryId ? 'library-group-chip active' : 'library-group-chip'}
              onClick={() => selectLibrary(adultLibraryId)}
              type="button"
            >
              全部
            </button>
            {adultGroups.map((group) => (
              <button
                aria-pressed={libraryId === group.id}
                className={libraryId === group.id ? 'library-group-chip active' : 'library-group-chip'}
                key={group.id}
                onClick={() => selectLibrary(group.id)}
                type="button"
              >
                {group.name}
              </button>
            ))}
          </nav>
        ) : null}

        {isExpanded ? (
          <div aria-label="全部媒体库" className="library-expand-panel" role="region">
            <div className="library-expand-header">
              <div className="library-expand-title">
                <LayoutGrid size={15} />
                <span>全部媒体库</span>
                <span className="library-expand-badge">{topLibraries.length} 个分类</span>
              </div>
              <button
                className="library-expand-close-btn"
                onClick={() => setIsExpanded(false)}
                type="button"
              >
                <ChevronUp size={14} />
                <span>收起</span>
              </button>
            </div>
            <div className="library-expand-grid">
              {topLibraries.map((library) => {
                const typeLabel = libraryCollectionLabel(library.collectionType)
                const name = libraryDisplayName(library.name)
                const isSelected = library.id === selectedRootId
                const Icon = getLibraryIcon(name, library.collectionType)
                return (
                  <button
                    className={isSelected ? 'library-expand-card active' : 'library-expand-card'}
                    key={library.id}
                    onClick={() => {
                      selectLibrary(library.id)
                      setIsExpanded(false)
                    }}
                    type="button"
                  >
                    <div className="expand-card-icon">
                      <Icon size={18} />
                    </div>
                    <div className="expand-card-body">
                      <strong className="expand-card-name">{name}</strong>
                      <span className="expand-card-type">{typeLabel}</span>
                    </div>
                    {isSelected ? (
                      <span className="expand-card-status">当前</span>
                    ) : null}
                  </button>
                )
              })}
            </div>
          </div>
        ) : null}

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
          <div aria-label="按媒体类型与状态筛选" className="filter-group" role="group">
            <button
              aria-label="筛选全部类型"
              aria-pressed={typeFilter === 'all'}
              className={typeFilter === 'all' ? 'filter-pill active' : 'filter-pill'}
              onClick={() => setTypeFilter('all')}
              type="button"
            >
              全部类型
            </button>
            <button
              aria-label="筛选电影"
              aria-pressed={typeFilter === 'Movie'}
              className={typeFilter === 'Movie' ? 'filter-pill active' : 'filter-pill'}
              onClick={() => setTypeFilter('Movie')}
              type="button"
            >
              仅电影
            </button>
            <button
              aria-label="筛选剧集"
              aria-pressed={typeFilter === 'Series'}
              className={typeFilter === 'Series' ? 'filter-pill active' : 'filter-pill'}
              onClick={() => setTypeFilter('Series')}
              type="button"
            >
              仅剧集
            </button>
          </div>
          <div aria-label="按观看状态筛选" className="filter-group" role="group">
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
                setSortBy(nextLibrarySort(sortBy, adultBrowsing))
                setPage(0)
              }}
              type="button"
            >
              {librarySortLabel(sortBy, adultBrowsing)}
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
            <LibraryPosterGridSkeleton count={12} />
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
            {items.map((item) => (
              <LibraryPosterCard
                inPagePlayback={inPagePlayback}
                item={item}
                key={item.id}
                onPlay={startPlay}
                onSelect={selectItem}
                selected={item.id === itemId}
              />
            ))}
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
          {detail.isLoading && !detail.data ? <LibraryDetailSkeleton /> : null}
          {detail.isError ? <div className="inline-error"><CircleAlert size={18} /><div><strong>详情读取失败</strong><span>{detail.error.message}</span></div><button onClick={() => void detail.refetch()} type="button">重试</button></div> : null}
          {detail.data ? <LibraryItemDetail inPagePlayback={inPagePlayback} item={detail.data} onDeleted={closeItem} onPlay={startPlay} onRefresh={(id) => refreshItem.mutate(id)} onSelectPerson={selectPerson} refreshing={refreshItem.isPending} seriesEpisodes={episodes.data?.items} shared={isSharedEmbyId(detail.data.id)} /> : null}
        </aside>
      ) : null}
      {inPagePlayback && playTarget ? (
        <LibraryPlayer
          externalUrl={playTarget.externalUrl}
          itemId={playTarget.id}
          onClose={closePlay}
          onSelectQueueItem={startQueueItem}
          queue={playerQueue}
          queueIsEpisodes={Boolean(playEpisode)}
          title={playTarget.title}
        />
      ) : null}
    </section>
  )
}

function LibraryPosterCard({
  item,
  selected,
  inPagePlayback,
  onSelect,
  onPlay,
}: {
  item: EmbyItem
  selected: boolean
  inPagePlayback: boolean
  onSelect: (id: string) => void
  onPlay: (target: { id: string }) => void
}) {
  const [failed, setFailed] = useState(false)
  const [imgLoaded, setImgLoaded] = useState(false)
  const isPlayed = Boolean(item.played)
  const isProgress = (item.playbackPositionMs ?? 0) >= 30_000 && !isPlayed
  const status = isPlayed || isProgress ? playbackStatus(item) : ''
  const playable = inPagePlayback && (item.type === 'Movie' || item.type === 'Video')

  return (
    <article className={selected ? 'library-poster-card selected' : 'library-poster-card'}>
      <div className="library-poster-hit">
        <div aria-hidden="true" className="library-poster-frame" onClick={() => onSelect(item.id)}>
          {!failed ? (
            <img
              alt=""
              className={imgLoaded ? 'fade-in-image loaded' : 'fade-in-image'}
              decoding="async"
              height={240}
              loading="lazy"
              onError={() => setFailed(true)}
              onLoad={() => setImgLoaded(true)}
              src={embyPrimaryImageURL(item.id)}
              width={160}
            />
          ) : (
            <span aria-hidden="true" className="library-poster-fallback"><Film size={28} /></span>
          )}
          <div aria-hidden="true" className="poster-vignette" />
          {playable ? <div aria-hidden="true" className="poster-hover-overlay" /> : null}
          {status ? (
            <span className={isPlayed ? 'library-poster-badge played' : 'library-poster-badge in-progress'}>
              {status}
            </span>
          ) : null}
        </div>
        {playable ? (
          <button
            aria-label={`播放 ${item.name}`}
            className="poster-play-icon"
            onClick={() => onPlay(item)}
            type="button"
          >
            <Play fill="currentColor" size={18} />
          </button>
        ) : null}
      </div>
      <button aria-pressed={selected} className="library-poster-meta" onClick={() => onSelect(item.id)} type="button">
        <strong className="poster-title">{item.name}</strong>
        <span className="poster-subtitle">
          <span className="poster-type">{mediaTypeLabel(item.type)}</span>
          {item.year ? <span className="poster-year">· {item.year}</span> : null}
        </span>
      </button>
    </article>
  )
}

function libraryCollectionLabel(type?: string) {
  if (type === 'movies') return '电影'
  if (type === 'tvshows') return '剧集'
  if (type === 'music') return '音乐'
  if (type === 'boxsets') return '合集'
  if (type === 'homevideos') return '视频'
  return '媒体库'
}

function getLibraryIcon(name: string, collectionType?: string) {
  const lowerName = name.toLowerCase()
  const lowerType = (collectionType ?? '').toLowerCase()
  if (lowerName.includes('音') || lowerName.includes('唱') || lowerType === 'music') {
    return Music
  }
  if (lowerName.includes('漫') || lowerName.includes('画') || lowerName.includes('anime')) {
    return Clapperboard
  }
  if (lowerName.includes('剧') || lowerName.includes('综') || lowerType === 'tvshows') {
    return Tv
  }
  if (lowerName.includes('纪录') || lowerName.includes('专栏') || lowerType === 'boxsets') {
    return BookOpen
  }
  return Film
}
