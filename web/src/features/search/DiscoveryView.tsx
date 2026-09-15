import { useEffect, useMemo, useRef, useState, type FormEvent, type ReactNode } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { BellPlus, CircleAlert, Download, Film, FolderInput, HardDrive, RefreshCw, Search, Star, Tv, Volume2, X } from 'lucide-react'
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
  type Integration,
} from '../../shared/api/mediaHub'
import { commitUrl } from '../../shared/navigation/urlState'
import { IconButton } from '../../shared/ui/IconButton'
import {
  discoveryFocusSubtitle,
  discoveryResultsHeading,
  discoverySearchQuery,
  prioritizeDiscoveryResults,
} from './discoverySearch'

function formatSize(bytes: number) {
  if (bytes <= 0) return '大小未知'
  const gib = bytes / 1024 / 1024 / 1024
  return gib >= 1024 ? `${(gib / 1024).toFixed(1)} TB` : `${gib.toFixed(1)} GB`
}

function searchStateFromLocation() {
  const params = new URLSearchParams(window.location.search)
  return {
    query: params.get('q')?.trim().slice(0, 120) ?? '',
    focusTitle: params.get('focus')?.trim().slice(0, 120) ?? '',
    focusSubtitle: params.get('focusMeta')?.trim().slice(0, 40) ?? '',
  }
}

type DiscoveryViewProps = {
  integrations: Integration[]
  integrationsLoading: boolean
  onRefreshIntegrations: () => void
  onTransferCreated: () => void
  onSubscribe: (candidate: Candidate) => void
}

export function DiscoveryView({
  integrations,
  integrationsLoading,
  onRefreshIntegrations,
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
  const sourceIntegration = integrations.find((item) => item.id === 'sources')
  const healthyCount = integrations.filter((item) => item.status === 'healthy').length

  const search = useQuery({
    queryKey: ['search', submittedQuery],
    queryFn: () => searchMedia(submittedQuery),
    enabled: Boolean(submittedQuery),
  })
  const trending = useQuery({
    queryKey: ['tmdb-trending'],
    queryFn: () => getTrending('all', 10),
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
    const focusSearch = (event: KeyboardEvent) => {
      const isInput = document.activeElement?.tagName === 'INPUT' || document.activeElement?.tagName === 'TEXTAREA' || document.activeElement?.tagName === 'SELECT'
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault()
        inputRef.current?.focus()
      } else if (event.key === '/' && !isInput) {
        event.preventDefault()
        inputRef.current?.focus()
      }
    }
    window.addEventListener('keydown', focusSearch)
    return () => window.removeEventListener('keydown', focusSearch)
  }, [])

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

  useEffect(() => {
    setResolutionFilter('all')
    setSortMode('default')
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
    return list
  }, [rankedResults, resolutionFilter, sortMode])

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

  return (
    <section className="discovery-view">
      <header className="view-header discovery-header">
        <div><h1>发现</h1><p>搜索资源、核对版本并加入自动转存流程。</p></div>
        <button aria-label={integrationsLoading ? '正在检查服务状态' : `刷新服务状态，${healthyCount}/${integrations.length} 个服务在线`} className="health-summary" disabled={integrationsLoading} onClick={onRefreshIntegrations} type="button"><span className={healthyCount > 0 ? 'healthy' : ''} /><strong>{integrationsLoading ? '检查中…' : `${healthyCount}/${integrations.length} 服务在线`}</strong><RefreshCw aria-hidden="true" size={15} /></button>
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
            <kbd aria-hidden="true" className="search-shortcut">⌘K</kbd>
          )}
          <button disabled={!query.trim() || search.isFetching} type="submit">{search.isFetching ? '查找中…' : '搜索'}</button>
        </form>
        <div className="search-meta"><span>{sourceIntegration?.detail ?? '资源源尚未配置'}</span>{submittedQuery ? <><span>·</span><span>{search.data?.partial ? '部分结果' : '已列出可获取版本'}</span></> : null}</div>
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
            {!search.isLoading && !search.isError && rankedResults.length > 0 ? (
              <div className="discovery-filter-bar">
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
                <button className="secondary-command" onClick={() => setResolutionFilter('all')} type="button">查看全部清晰度</button>
              </div>
            ) : null}
            {!search.isLoading && !search.isError ? filteredResults.map((candidate) => {
              const transferring = (transfer.isPending && transfer.variables?.id === candidate.id) ||
                queuedTransferIds.includes(candidate.id) ||
                candidate.transferState === 'transferring'
              const downloadable = isDownloadable(candidate)
              const available = (candidate.transferState === 'available' || downloadable) && Boolean(candidate.transferToken) && !transferring
              const subscribable = candidate.transferState !== 'identity_required' && Boolean(candidate.tmdbId)
              const availabilityLabel = transferring
                ? (downloadable ? '下载中' : '转存中')
                : available
                  ? (downloadable ? '可下载' : '可转存')
                  : candidate.transferState === 'identity_required'
                    ? '身份待确认'
                    : '工作流不可用'
              return (
                <article className={selected?.id === candidate.id ? 'result-row selected' : 'result-row'} data-candidate-id={candidate.id} key={candidate.id}>
                  <div className="poster-wrap">{candidate.posterUrl ? <img alt={`${candidate.title} 海报`} height="144" loading="lazy" src={candidate.posterUrl} width="96" /> : <Film aria-hidden="true" size={24} />}<span>{candidate.mediaType === 'movie' ? '电影' : '剧集'}</span></div>
                  <button aria-pressed={selected?.id === candidate.id} className="result-main result-select" onClick={() => setSelected(candidate)} type="button">
                    <span className="result-title-row"><span className="result-title">{candidate.title}</span><span className="year">{candidate.year}</span></span>
                    <span className="source-line">
                      <span className="spec-badge spec-source">{candidate.provider ?? candidate.source}</span>
                      {episodeLabel(candidate) ? <span className="spec-badge spec-episode">{episodeLabel(candidate)}</span> : null}
                      <span className={`spec-badge spec-res ${resolutionBadgeClass(candidate.release.resolution)}`}>{candidate.release.resolution}</span>
                      <span className="spec-badge spec-codec">{candidate.release.videoCodec}</span>
                      {candidate.release.dynamicRange ? <span className={`spec-badge spec-hdr ${dynamicRangeBadgeClass(candidate.release.dynamicRange)}`}>{candidate.release.dynamicRange}</span> : null}
                    </span>
                    <span className="result-facts">
                      <span className="result-fact-item"><Volume2 aria-hidden="true" size={13} /><span>{candidate.release.audio ?? '音轨未知'}</span></span>
                      <span>·</span>
                      <span className="result-fact-item"><HardDrive aria-hidden="true" size={13} /><span>{formatSize(candidate.release.sizeBytes)}</span></span>
                    </span>
                  </button>
                  <div className="result-action"><span className={`availability ${transferring ? 'transferring' : available ? 'available' : 'unavailable'}`}><span />{availabilityLabel}</span><div className="result-commands"><IconButton disabled={!subscribable} label="创建订阅" onClick={() => onSubscribe(candidate)}><BellPlus size={15} /></IconButton><button disabled={!available || transferring} onClick={() => handleTransfer(candidate)} type="button">{transferring ? (transfer.isPending && transfer.variables?.id === candidate.id ? '提交中…' : downloadable ? '下载中' : '转存中') : downloadable ? '下载' : '转存'}{downloadable ? <Download size={15} /> : <FolderInput size={15} />}</button></div></div>
                </article>
              )
            }) : null}
          </section>

          {selected ? <aside className="detail-panel" aria-label="资源详情">
            <div className="detail-header"><div><h2>{selected.title}</h2></div><IconButton label="关闭详情" onClick={() => setSelected(null)} subtle><X size={17} /></IconButton></div>
            <div className="detail-poster">
              {selected.posterUrl ? (
                <>
                  <div
                    aria-hidden="true"
                    className="detail-poster-backdrop"
                    style={{ backgroundImage: `url(${selected.posterUrl})` }}
                  />
                  <div aria-hidden="true" className="detail-poster-vignette" />
                  <div className="detail-poster-foreground">
                    <img alt={`${selected.title} 海报`} height="180" src={selected.posterUrl} width="120" />
                  </div>
                </>
              ) : (
                <div className="poster-placeholder"><Film size={34} /></div>
              )}
              <div className="poster-overlay">
                <span className="spec-badge spec-source">{selected.provider ?? selected.source}</span>
                <strong className={`spec-badge spec-res ${resolutionBadgeClass(selected.release.resolution)}`}>
                  {selected.release.resolution}
                </strong>
              </div>
            </div>
            <div className="detail-facts">
              <div className="detail-fact-card">
                <span><Tv aria-hidden="true" size={13} />视频规格</span>
                <strong>
                  {selected.release.videoCodec}
                  {selected.release.dynamicRange ? (
                    <> · <span className={`spec-badge spec-hdr ${dynamicRangeBadgeClass(selected.release.dynamicRange)}`}>{selected.release.dynamicRange}</span></>
                  ) : null}
                </strong>
              </div>
              <div className="detail-fact-card">
                <span><Volume2 aria-hidden="true" size={13} />音频规格</span>
                <strong>{selected.release.audio ?? '音轨未知'}</strong>
              </div>
              <div className="detail-fact-card">
                <span><HardDrive aria-hidden="true" size={13} />资源体积</span>
                <strong>{formatSize(selected.release.sizeBytes)}</strong>
              </div>
              <div className="detail-fact-card">
                <span>{isDownloadable(selected) ? <Download aria-hidden="true" size={13} /> : <FolderInput aria-hidden="true" size={13} />}{isDownloadable(selected) ? '下载目标' : '转存目标'}</span>
                <strong>{isDownloadable(selected) ? 'MoviePilot / 本地' : selected.mediaType === 'movie' ? '115 / 电影' : '115 / 电视剧'}</strong>
              </div>
            </div>
            {recommendations.data?.items.length ? <div className="recommendation-links"><span>相似内容</span>{recommendations.data.items.slice(0, 4).map((item) => <button key={`${item.mediaType}-${item.tmdbId}`} onClick={() => openDiscoveryItem(item)} type="button">{item.title}</button>)}</div> : null}
            <div className="detail-actions"><button className="secondary-command" disabled={!selected.tmdbId || selected.transferState === 'identity_required'} onClick={() => onSubscribe(selected)} type="button"><BellPlus size={16} />订阅</button><button className="primary-action" disabled={!selected.transferToken || transfer.isPending} onClick={() => handleTransfer(selected)} type="button">{isDownloadable(selected) ? <Download size={17} /> : <FolderInput size={17} />}{transfer.isPending ? '正在创建任务…' : selected.transferToken ? (isDownloadable(selected) ? '加入下载队列' : '加入转存队列') : selected.transferState === 'identity_required' ? '身份待确认' : '工作流不可用'}</button></div>
          </aside> : null}
        </div>
      ) : null}
    </section>
  )
}

function isDownloadable(candidate: Candidate) {
  return candidate.transferState === 'downloadable' || candidate.sourceId === 'moviepilot'
}

function episodeLabel(candidate: Candidate) {
  if (!candidate.season) return ''
  if (!candidate.episodeStart) return `S${candidate.season}`
  return candidate.episodeStart === candidate.episodeEnd
    ? `S${candidate.season}E${candidate.episodeStart}`
    : `S${candidate.season}E${candidate.episodeStart}-${candidate.episodeEnd}`
}

function resolutionBadgeClass(res: string) {
  const lower = res.toLowerCase()
  if (lower.includes('2160') || lower.includes('4k')) return 'badge-res-4k'
  if (lower.includes('1080')) return 'badge-res-1080p'
  return 'badge-res-default'
}

function dynamicRangeBadgeClass(hdr?: string) {
  if (!hdr) return ''
  const upper = hdr.toUpperCase()
  if (upper.includes('DV') || upper.includes('DOLBY')) return 'badge-hdr-dv'
  if (upper.includes('HDR')) return 'badge-hdr-standard'
  return 'badge-hdr-default'
}

function DiscoveryPosterBand({
  label,
  subtitle,
  items,
  onSelect,
  icon,
  loading = false,
  emptyLabel,
}: {
  label: string
  subtitle: string
  items: DiscoveryItem[]
  onSelect: (item: DiscoveryItem) => void
  icon?: ReactNode
  loading?: boolean
  emptyLabel?: string
}) {
  if (!loading && items.length === 0 && !emptyLabel) return null
  return (
    <section className="trending-band" aria-label={label}>
      <div className="section-heading">
        <div>
          <h2>{icon ? <span className="section-heading-icon">{icon}</span> : null}{label}</h2>
          <p>{subtitle}</p>
        </div>
      </div>
      {loading ? <p className="discovery-band-status" role="status">{emptyLabel ?? '正在加载…'}</p> : null}
      {!loading && items.length === 0 && emptyLabel ? (
        <p className="discovery-band-status" role="status">{emptyLabel}</p>
      ) : null}
      {items.length ? (
        <div className="trending-list">
          {items.map((item) => (
            <button key={`${item.mediaType}-${item.tmdbId}`} onClick={() => onSelect(item)} type="button">
              <span className="trending-poster">
                {item.posterUrl ? (
                  <img alt="" height="210" loading="lazy" src={item.posterUrl} width="140" />
                ) : (
                  <Film aria-hidden="true" size={20} />
                )}
              </span>
              <span>
                <strong>{item.title}</strong>
                <small>
                  {item.year || '年份未知'} · {item.mediaType === 'movie' ? '电影' : '剧集'}
                </small>
              </span>
            </button>
          ))}
        </div>
      ) : null}
    </section>
  )
}
