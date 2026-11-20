import { useEffect, useMemo, useRef, useState, type FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { CircleAlert, Film, Search, Star, Tv, X } from 'lucide-react'
import {
  createTransfer,
  getDiscoveryCatalog,
  getDiscoveryGenres,
  getRecommendations,
  getTrending,
  searchMedia,
  type Candidate,
  type DiscoveryGenre,
  type DiscoveryItem,
} from '../../shared/api/mediaHub'
import { commitUrl } from '../../shared/navigation/urlState'
import { PageCaptureForm } from './PageCaptureForm'
import { DiscoveryDetailPanel } from './DiscoveryDetailPanel'
import { DiscoveryPosterBand, ResultLane } from './DiscoveryResults'
import {
  discoveryFocusSubtitle,
  discoveryResultsHeading,
  discoverySearchQuery,
  isDownloadable,
  matchesPipeline,
  pickSearchIdentity,
  prioritizeDiscoveryResults,
  type PipelineLane,
} from './discoverySearch'

function searchStateFromLocation() {
  const params = new URLSearchParams(window.location.search)
  return {
    query: params.get('q')?.trim().slice(0, 120) ?? '',
    focusTitle: params.get('focus')?.trim().slice(0, 120) ?? '',
    focusSubtitle: params.get('focusMeta')?.trim().slice(0, 40) ?? '',
  }
}

type DiscoveryViewProps = {
  onTransferCreated: () => void
  onSubscribe: (candidate: Candidate) => void
}

export function DiscoveryView({
  onTransferCreated,
  onSubscribe,
}: DiscoveryViewProps) {
  const queryClient = useQueryClient()
  const inputRef = useRef<HTMLInputElement>(null)
  const initial = searchStateFromLocation()
  const [query, setQuery] = useState(initial.query)
  const [submittedQuery, setSubmittedQuery] = useState(initial.query)
  const [focusTitle, setFocusTitle] = useState(initial.focusTitle)
  const [focusSubtitle, setFocusSubtitle] = useState(initial.focusSubtitle)
  const [focusItem, setFocusItem] = useState<Pick<DiscoveryItem, 'tmdbId' | 'year' | 'mediaType'> | null>(null)
  const [selected, setSelected] = useState<Candidate | null>(null)

  const search = useQuery({
    queryKey: ['search', submittedQuery],
    queryFn: () => searchMedia(submittedQuery),
    enabled: Boolean(submittedQuery),
  })
  const trending = useQuery({
    queryKey: ['tmdb-trending'],
    queryFn: () => getTrending('all', 12),
    retry: false,
    staleTime: 5 * 60_000,
    gcTime: 30 * 60_000,
  })
  const topRatedMovies = useQuery({
    queryKey: ['discovery-catalog', 'top_rated', 'movie'],
    queryFn: () => getDiscoveryCatalog('top_rated', 'movie', { limit: 12 }),
    retry: false,
    staleTime: 5 * 60_000,
    gcTime: 30 * 60_000,
  })
  const popularSeries = useQuery({
    queryKey: ['discovery-catalog', 'popular', 'series'],
    queryFn: () => getDiscoveryCatalog('popular', 'series', { limit: 12 }),
    retry: false,
    staleTime: 5 * 60_000,
    gcTime: 30 * 60_000,
  })
  const movieGenres = useQuery({
    queryKey: ['discovery-genres', 'movie'],
    queryFn: () => getDiscoveryGenres('movie'),
    retry: false,
    staleTime: 60 * 60_000,
    gcTime: 2 * 60 * 60_000,
  })
  const [selectedGenre, setSelectedGenre] = useState<DiscoveryGenre | null>(null)
  const genreBrowse = useQuery({
    queryKey: ['discovery-catalog', 'genre', 'movie', selectedGenre?.id],
    queryFn: () => getDiscoveryCatalog('genre', 'movie', { genreId: selectedGenre!.id, limit: 16 }),
    enabled: Boolean(selectedGenre),
    retry: false,
    staleTime: 5 * 60_000,
    gcTime: 30 * 60_000,
  })
  const [queuedTransferIds, setQueuedTransferIds] = useState<string[]>([])
  const transfer = useMutation({
    mutationFn: (candidate: Candidate) => {
      if (!candidate.transferToken) throw new Error(isDownloadable(candidate) ? '当前资源不能下载' : '当前资源不能转存')
      return createTransfer(candidate.transferToken, crypto.randomUUID())
    },
    onSuccess: async (_data, candidate) => {
      setQueuedTransferIds((current) => current.includes(candidate.id) ? current : [...current, candidate.id])
      await queryClient.invalidateQueries({ queryKey: ['transfers'] })
      onTransferCreated()
    },
  })

  useEffect(() => {
    const restoreSearch = () => {
      const restored = searchStateFromLocation()
      setQuery(restored.query)
      setSubmittedQuery(restored.query)
      setFocusTitle(restored.focusTitle)
      setFocusSubtitle(restored.focusSubtitle)
      setFocusItem(null)
      setSelected(null)
    }
    window.addEventListener('popstate', restoreSearch)
    return () => window.removeEventListener('popstate', restoreSearch)
  }, [])

  const recommendations = useQuery({
    queryKey: ['tmdb-recommendations', selected?.mediaType, selected?.tmdbId],
    queryFn: () => getRecommendations(selected!.mediaType, selected!.tmdbId!, 6),
    enabled: Boolean(selected?.tmdbId),
    retry: false,
    staleTime: 5 * 60_000,
    gcTime: 30 * 60_000,
  })

  const [resolutionFilter, setResolutionFilter] = useState<'all' | '2160p' | '1080p' | 'other'>('all')
  const [sortMode, setSortMode] = useState<'default' | 'size-desc' | 'size-asc'>('default')
  const [pipelineFilter, setPipelineFilter] = useState<PipelineLane>('all')
  const [focusIdentityId, setFocusIdentityId] = useState<string | null>(null)

  useEffect(() => {
    setResolutionFilter('all')
    setSortMode('default')
    setPipelineFilter('all')
    setFocusIdentityId(null)
  }, [submittedQuery])

  const rankedResults = useMemo(() => {
    const results = search.data?.results ?? []
    return focusItem ? prioritizeDiscoveryResults(results, focusItem) : results
  }, [focusItem, search.data?.results])

  const filteredResults = useMemo(() => {
    let list = rankedResults
    if (resolutionFilter === '2160p') {
      list = list.filter((item) => /2160p|4k/i.test(item.release.resolution))
    } else if (resolutionFilter === '1080p') {
      list = list.filter((item) => /1080p/i.test(item.release.resolution))
    } else if (resolutionFilter === 'other') {
      list = list.filter((item) => !/2160p|4k|1080p/i.test(item.release.resolution))
    }

    if (sortMode === 'size-desc') {
      list = [...list].sort((a, b) => (b.release.sizeBytes || 0) - (a.release.sizeBytes || 0))
    } else if (sortMode === 'size-asc') {
      list = [...list].sort((a, b) => (a.release.sizeBytes || 0) - (b.release.sizeBytes || 0))
    }
    if (pipelineFilter !== 'all') {
      list = list.filter((item) => matchesPipeline(item, pipelineFilter))
    }
    return list
  }, [rankedResults, resolutionFilter, sortMode, pipelineFilter])

  useEffect(() => {
    if (!search.isSuccess) return
    if (selected && filteredResults.some((item) => item.id === selected.id)) return
    setSelected(filteredResults[0] ?? null)
  }, [search.dataUpdatedAt, filteredResults, search.isSuccess, selected])

  useEffect(() => {
    const handleNavigationKeydown = (event: KeyboardEvent) => {
      const activeTag = document.activeElement?.tagName
      const isEditing = activeTag === 'INPUT' || activeTag === 'TEXTAREA' || activeTag === 'SELECT'
      if (isEditing) return
      if (event.metaKey || event.ctrlKey || event.altKey) return

      if (!submittedQuery || filteredResults.length === 0) return

      if (event.key === 'ArrowDown' || event.key === 'j') {
        event.preventDefault()
        setSelected((current) => {
          const idx = current ? filteredResults.findIndex((item) => item.id === current.id) : -1
          const nextIdx = idx < filteredResults.length - 1 ? idx + 1 : 0
          const next = filteredResults[nextIdx] ?? null
          if (next) {
            document.querySelector<HTMLElement>(`[data-candidate-id="${next.id}"]`)?.scrollIntoView({ block: 'nearest' })
          }
          return next
        })
      } else if (event.key === 'ArrowUp' || event.key === 'k') {
        event.preventDefault()
        setSelected((current) => {
          const idx = current ? filteredResults.findIndex((item) => item.id === current.id) : -1
          const prevIdx = idx > 0 ? idx - 1 : filteredResults.length - 1
          const prev = filteredResults[prevIdx] ?? null
          if (prev) {
            document.querySelector<HTMLElement>(`[data-candidate-id="${prev.id}"]`)?.scrollIntoView({ block: 'nearest' })
          }
          return prev
        })
      } else if (event.key === 'Escape') {
        if (selected) {
          event.preventDefault()
          setSelected(null)
        }
      }
    }

    window.addEventListener('keydown', handleNavigationKeydown)
    return () => window.removeEventListener('keydown', handleNavigationKeydown)
  }, [submittedQuery, filteredResults, selected])

  const handleSearch = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const next = query.trim()
    if (!next) return
    setSubmittedQuery(next)
    setFocusTitle('')
    setFocusSubtitle('')
    setFocusItem(null)
    setSelected(null)
    commitUrl({ q: next, focus: null, focusMeta: null })
  }

  const openDiscoveryItem = (item: DiscoveryItem) => {
    const next = discoverySearchQuery(item)
    if (!next) return
    setQuery(next)
    setSubmittedQuery(next)
    setFocusTitle(item.title)
    setFocusSubtitle(discoveryFocusSubtitle(item))
    setFocusItem({ tmdbId: item.tmdbId, year: item.year, mediaType: item.mediaType })
    setSelected(null)
    commitUrl({
      q: next,
      focus: item.title,
      focusMeta: discoveryFocusSubtitle(item),
    })
  }

  const handleTransfer = (candidate: Candidate) => {
    setSelected(candidate)
    transfer.mutate(candidate)
  }

  const heading = discoveryResultsHeading(search.isFetching, focusTitle)
  const identities = search.data?.identities ?? []
  const titleIdentity = focusIdentityId
    ? identities.find((item) => item.tmdbId === focusIdentityId) ?? pickSearchIdentity(identities, selected)
    : pickSearchIdentity(identities, selected)
  const transferCount = rankedResults.filter((item) => !isDownloadable(item)).length
  const downloadCount = rankedResults.filter((item) => isDownloadable(item)).length
  const showPipelineGroups = pipelineFilter === 'all' && transferCount > 0 && downloadCount > 0
  const detailOverview = selected?.overview || titleIdentity?.overview
  const detailRating = selected?.rating || titleIdentity?.rating
  const detailPoster = selected?.posterUrl || titleIdentity?.posterUrl

  return (
    <section className="discovery-view">
      <header className="view-header discovery-header">
        <div><h1>发现</h1></div>
      </header>

      <section className="search-stage" aria-label="搜索媒体资源">
        <form className="search-form" onSubmit={handleSearch}>
          <Search size={21} aria-hidden="true" />
          <label className="sr-only" htmlFor="media-search">搜索电影或电视剧</label>
          <input
            autoComplete="off"
            id="media-search"
            maxLength={120}
            name="media-query"
            onChange={(event) => {
              setQuery(event.target.value)
              if (focusTitle) {
                setFocusTitle('')
                setFocusSubtitle('')
                setFocusItem(null)
              }
            }}
            placeholder="搜索电影或电视剧…"
            ref={inputRef}
            value={query}
          />
          {query ? (
            <button
              aria-label="清空搜索内容"
              className="search-clear-button"
              onClick={() => {
                setQuery('')
                inputRef.current?.focus()
              }}
              type="button"
            >
              <X aria-hidden="true" size={16} />
            </button>
          ) : (
            <kbd aria-hidden="true" className="search-shortcut">/</kbd>
          )}
          <button disabled={!query.trim() || search.isFetching} type="submit">{search.isFetching ? '查找中…' : '搜索'}</button>
        </form>
        {submittedQuery ? <div className="search-meta"><span>{search.data?.partial ? '部分结果' : '已列出可获取版本'}</span></div> : null}
        <PageCaptureForm onImported={onTransferCreated} />
      </section>

      {!submittedQuery ? (
        <div className="discovery-idle">
          {movieGenres.data?.genres?.length ? (
            <section className="genre-chip-band" aria-label="电影类型">
              <div className="genre-chip-row" role="group" aria-label="筛选类型">
                <button
                  aria-pressed={!selectedGenre}
                  className={!selectedGenre ? 'genre-chip selected' : 'genre-chip'}
                  onClick={() => setSelectedGenre(null)}
                  type="button"
                >
                  全部类型
                </button>
                {movieGenres.data.genres.map((genre) => {
                  const selected = selectedGenre?.id === genre.id
                  return (
                    <button
                      aria-pressed={selected}
                      className={selected ? 'genre-chip selected' : 'genre-chip'}
                      key={genre.id}
                      onClick={() => setSelectedGenre(selected ? null : genre)}
                      type="button"
                    >
                      {genre.name}
                    </button>
                  )
                })}
              </div>
            </section>
          ) : null}

          {selectedGenre ? (
            <DiscoveryPosterBand
              emptyLabel={genreBrowse.isFetching ? '正在加载…' : `暂无「${selectedGenre.name}」片源`}
              items={genreBrowse.data?.items ?? []}
              label={`${selectedGenre.name}片`}
              loading={genreBrowse.isFetching && !genreBrowse.data?.items.length}
              onSelect={openDiscoveryItem}
              subtitle="点选后直接列出可获取版本"
            />
          ) : null}

          <DiscoveryPosterBand
            icon={<Star aria-hidden="true" size={16} />}
            items={topRatedMovies.data?.items ?? []}
            label="高分电影"
            onSelect={openDiscoveryItem}
            subtitle="TMDB 评分靠前 · 点选直接列出可获取版本"
          />

          <DiscoveryPosterBand
            icon={<Tv aria-hidden="true" size={16} />}
            items={popularSeries.data?.items ?? []}
            label="热门剧集"
            onSelect={openDiscoveryItem}
            subtitle="TMDB 热度靠前 · 点选直接列出可获取版本"
          />

          {trending.data?.items.length ? (
            <DiscoveryPosterBand
              items={trending.data.items}
              label="本周热门"
              onSelect={openDiscoveryItem}
              subtitle="点选后直接列出可获取版本"
            />
          ) : null}
        </div>
      ) : null}

      {search.data?.sourceErrors.length ? <div className="source-warning" role="status"><CircleAlert size={16} /><span>{search.data.sourceErrors.map((item) => item.message).join(' · ')}</span></div> : null}
      {transfer.isError ? <div className="source-warning error" role="alert"><CircleAlert size={16} /><span>{transfer.error.message}</span></div> : null}

      {submittedQuery ? (
        <div className={selected ? 'content-grid has-detail' : 'content-grid'}>
          <section className="results-column">
            <div className="section-heading">
              <div>
                <h2>{heading}{!search.isFetching ? <span>{filteredResults.length}</span> : null}</h2>
                {focusSubtitle ? <p>{focusSubtitle}</p> : null}
              </div>
            </div>
            {titleIdentity ? (
              <div className="search-identity-card">
                <div className="search-identity-poster">
                  {titleIdentity.posterUrl ? (
                    <img alt={`${titleIdentity.title} 海报`} height="150" src={titleIdentity.posterUrl} width="100" />
                  ) : (
                    <Film aria-hidden="true" size={24} />
                  )}
                </div>
                <div className="search-identity-copy">
                  <strong>{titleIdentity.title}</strong>
                  <span>
                    {titleIdentity.year || '年份未知'} · {titleIdentity.mediaType === 'series' ? '剧集' : '电影'}
                    {titleIdentity.rating ? ` · ${titleIdentity.rating.toFixed(1)}` : ''}
                  </span>
                  {titleIdentity.overview ? <p>{titleIdentity.overview}</p> : null}
                  {identities.length > 1 ? (
                    <div aria-label="匹配片名" className="search-identity-picker" role="group">
                      {identities.map((item) => (
                        <button
                          aria-pressed={(focusIdentityId ?? titleIdentity.tmdbId) === item.tmdbId}
                          className={(focusIdentityId ?? titleIdentity.tmdbId) === item.tmdbId ? 'filter-chip active' : 'filter-chip'}
                          key={item.tmdbId}
                          onClick={() => setFocusIdentityId(item.tmdbId)}
                          type="button"
                        >
                          {item.year || item.title}
                        </button>
                      ))}
                    </div>
                  ) : null}
                </div>
              </div>
            ) : null}
            {!search.isLoading && !search.isError && rankedResults.length > 0 ? (
              <div className="discovery-filter-bar">
                {transferCount > 0 && downloadCount > 0 ? (
                  <div aria-label="按获取方式筛选" className="discovery-filter-group" role="group">
                    <button
                      aria-pressed={pipelineFilter === 'all'}
                      className={pipelineFilter === 'all' ? 'filter-chip active' : 'filter-chip'}
                      onClick={() => setPipelineFilter('all')}
                      type="button"
                    >
                      全部 {rankedResults.length}
                    </button>
                    <button
                      aria-pressed={pipelineFilter === 'transfer'}
                      className={pipelineFilter === 'transfer' ? 'filter-chip active' : 'filter-chip'}
                      onClick={() => setPipelineFilter('transfer')}
                      type="button"
                    >
                      115 转存 {transferCount}
                    </button>
                    <button
                      aria-pressed={pipelineFilter === 'download'}
                      className={pipelineFilter === 'download' ? 'filter-chip active' : 'filter-chip'}
                      onClick={() => setPipelineFilter('download')}
                      type="button"
                    >
                      PT 下载 {downloadCount}
                    </button>
                  </div>
                ) : null}
                <div aria-label="按清晰度筛选" className="discovery-filter-group" role="group">
                  <button
                    aria-pressed={resolutionFilter === 'all'}
                    className={resolutionFilter === 'all' ? 'filter-chip active' : 'filter-chip'}
                    onClick={() => setResolutionFilter('all')}
                    type="button"
                  >
                    全部清晰度
                  </button>
                  <button
                    aria-pressed={resolutionFilter === '2160p'}
                    className={resolutionFilter === '2160p' ? 'filter-chip active' : 'filter-chip'}
                    onClick={() => setResolutionFilter('2160p')}
                    type="button"
                  >
                    4K / 2160p
                  </button>
                  <button
                    aria-pressed={resolutionFilter === '1080p'}
                    className={resolutionFilter === '1080p' ? 'filter-chip active' : 'filter-chip'}
                    onClick={() => setResolutionFilter('1080p')}
                    type="button"
                  >
                    1080p
                  </button>
                </div>
                <div className="discovery-sort-group">
                  <label className="sr-only" htmlFor="discovery-sort-select">排序方式</label>
                  <select
                    className="discovery-sort-select"
                    id="discovery-sort-select"
                    onChange={(event) => setSortMode(event.target.value as 'default' | 'size-desc' | 'size-asc')}
                    value={sortMode}
                  >
                    <option value="default">默认推荐排序</option>
                    <option value="size-desc">体积从大到小</option>
                    <option value="size-asc">体积从小到大</option>
                  </select>
                </div>
                <span aria-hidden="true" className="discovery-keyboard-hint">
                  <kbd>J</kbd><kbd>K</kbd> 切换 · <kbd>Esc</kbd> 关闭
                </span>
              </div>
            ) : null}
            {search.isLoading ? <div className="result-loading"><div /><div /><div /></div> : null}
            {search.isError ? <div className="inline-error"><CircleAlert size={18} /><div><strong>暂时无法列出可获取版本</strong><span>{search.error.message}</span></div><button onClick={() => void search.refetch()} type="button">重试</button></div> : null}
            {!search.isLoading && !search.isError && rankedResults.length === 0 ? (
              <div className="empty-state">
                <Film size={26} />
                <span>{focusTitle ? `没有找到《${focusTitle}》的可获取版本` : '没有找到匹配资源'}</span>
              </div>
            ) : null}
            {!search.isLoading && !search.isError && rankedResults.length > 0 && filteredResults.length === 0 ? (
              <div className="empty-state">
                <Film size={26} />
                <span>当前筛选下没有匹配的版本</span>
                <button
                  className="secondary-command"
                  onClick={() => {
                    setResolutionFilter('all')
                    setPipelineFilter('all')
                  }}
                  type="button"
                >
                  重置筛选
                </button>
              </div>
            ) : null}
            {!search.isLoading && !search.isError ? (
              showPipelineGroups ? (
                <>
                  <ResultLane fallbackPoster={identities.length === 1 ? identities[0]?.posterUrl : undefined} heading="115 转存" items={filteredResults.filter((item) => !isDownloadable(item))} queuedTransferIds={queuedTransferIds} selected={selected} transfer={transfer} onSelect={setSelected} onSubscribe={onSubscribe} onTransfer={handleTransfer} />
                  <ResultLane fallbackPoster={identities.length === 1 ? identities[0]?.posterUrl : undefined} heading="PT 下载" items={filteredResults.filter((item) => isDownloadable(item))} queuedTransferIds={queuedTransferIds} selected={selected} transfer={transfer} onSelect={setSelected} onSubscribe={onSubscribe} onTransfer={handleTransfer} />
                </>
              ) : (
                <ResultLane fallbackPoster={identities.length === 1 ? identities[0]?.posterUrl : undefined} heading={pipelineFilter === 'download' ? 'PT 下载' : pipelineFilter === 'transfer' ? '115 转存' : ''} items={filteredResults} queuedTransferIds={queuedTransferIds} selected={selected} transfer={transfer} onSelect={setSelected} onSubscribe={onSubscribe} onTransfer={handleTransfer} />
              )
            ) : null}
          </section>

          {selected ? (
            <DiscoveryDetailPanel
              detailOverview={detailOverview}
              detailPoster={detailPoster}
              detailRating={detailRating}
              isTransferPending={transfer.isPending}
              onClose={() => setSelected(null)}
              onOpenDiscoveryItem={openDiscoveryItem}
              onSubscribe={onSubscribe}
              onTransfer={handleTransfer}
              recommendations={recommendations.data?.items ?? []}
              selected={selected}
            />
          ) : null}
        </div>
      ) : null}
    </section>
  )
}
