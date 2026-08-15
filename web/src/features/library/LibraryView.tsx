import { useEffect, useState, type FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { BookOpen, ChevronLeft, ChevronRight, CircleAlert, ExternalLink, Film, FolderOpen, RefreshCw, Search, X } from 'lucide-react'
import {
  getEmbyItem,
  getEmbyLibraries,
  getEmbyLibraryItems,
  refreshEmbyItem,
  refreshEmbyLibrary,
  searchEmbyItems,
  type EmbyItem,
  type EmbyItemDetail,
} from '../../shared/api/mediaHub'
import { commitUrl } from '../../shared/navigation/urlState'
import { IconButton } from '../../shared/ui/IconButton'

const pageSize = 24

function locationValue(name: string) {
  return new URLSearchParams(window.location.search).get(name)
}

export function LibraryView() {
  const queryClient = useQueryClient()
  const [libraryId, setLibraryId] = useState<string | null>(() => locationValue('library'))
  const [itemId, setItemId] = useState<string | null>(() => locationValue('media'))
  const [page, setPage] = useState(0)
  const [queryText, setQueryText] = useState('')
  const [submittedQuery, setSubmittedQuery] = useState('')

  const libraries = useQuery({ queryKey: ['emby-libraries'], queryFn: getEmbyLibraries })
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
      setLibraryId(locationValue('library'))
      setItemId(locationValue('media'))
      setPage(0)
      setQueryText('')
      setSubmittedQuery('')
    }
    window.addEventListener('popstate', restoreLocation)
    return () => window.removeEventListener('popstate', restoreLocation)
  }, [])

  useEffect(() => {
    if (!libraries.data) return
    const available = libraries.data.libraries
    if (libraryId && available.some((library) => library.id === libraryId)) return
    const fallback = available[0]?.id ?? null
    setLibraryId(fallback)
    setItemId(null)
    setPage(0)
    commitUrl({ library: fallback, media: null }, 'replace')
  }, [libraries.data, libraryId])

  const selectLibrary = (id: string) => {
    setLibraryId(id)
    setItemId(null)
    setPage(0)
    setQueryText('')
    setSubmittedQuery('')
    commitUrl({ library: id, media: null })
  }
  const selectItem = (id: string) => {
    setItemId(id)
    commitUrl({ media: id })
  }
  const closeItem = () => {
    setItemId(null)
    commitUrl({ media: null })
  }
  const submitSearch = (event: FormEvent) => {
    event.preventDefault()
    const value = queryText.trim()
    setSubmittedQuery(value)
    setItemId(null)
    commitUrl({ media: null })
  }
  const clearSearch = () => {
    setQueryText('')
    setSubmittedQuery('')
    setItemId(null)
    commitUrl({ media: null })
  }
  const changePage = (nextPage: number) => {
    setPage(nextPage)
    setItemId(null)
    commitUrl({ media: null })
  }

  const result = submittedQuery ? search : libraryItems
  const items = result.data?.items ?? []
  const total = result.data?.total ?? 0
  const selectedLibrary = libraries.data?.libraries.find((library) => library.id === libraryId)
  const mutationError = refreshLibrary.error ?? refreshItem.error

  return (
    <section className="library-page">
      <header className="view-header compact-view-header">
        <div><h1>媒体库</h1><p>浏览已入库内容，查看媒体信息并按需刷新元数据。</p></div>
        <IconButton label="刷新媒体库列表" onClick={() => void libraries.refetch()} subtle><RefreshCw size={17} /></IconButton>
      </header>

      {libraries.isError ? <div className="inline-error"><CircleAlert size={18} /><div><strong>媒体库读取失败</strong><span>{libraries.error.message}</span></div><button onClick={() => void libraries.refetch()} type="button">重试</button></div> : null}
      {mutationError ? <div className="source-warning error" role="alert"><CircleAlert size={16} /><span>{mutationError.message}</span></div> : null}

      <nav className="library-selector" aria-label="Emby 媒体库">
        {libraries.data?.libraries.map((library) => {
          const typeLabel = libraryCollectionLabel(library.collectionType)
          return (
            <button aria-pressed={library.id === libraryId} className={library.id === libraryId ? 'library-button selected' : 'library-button'} key={library.id} onClick={() => selectLibrary(library.id)} type="button">
              <FolderOpen size={18} /><span><strong>{library.name}</strong>{typeLabel.toLocaleLowerCase() !== library.name.trim().toLocaleLowerCase() ? <small>{typeLabel}</small> : null}</span>
            </button>
          )
        })}
      </nav>

      <div className="library-toolbar">
        <form className="library-search" onSubmit={submitSearch}>
          <Search size={17} aria-hidden="true" />
          <label className="sr-only" htmlFor="library-query">搜索 Emby 媒体</label>
          <input id="library-query" maxLength={120} onChange={(event) => setQueryText(event.target.value)} placeholder="搜索电影或剧集" type="search" value={queryText} />
          {submittedQuery ? <IconButton label="清除搜索" onClick={clearSearch} subtle><X size={16} /></IconButton> : null}
          <button className="primary-action compact" disabled={!queryText.trim()} type="submit">搜索</button>
        </form>
        {selectedLibrary && !submittedQuery ? <button className="secondary-command" disabled={refreshLibrary.isPending} onClick={() => refreshLibrary.mutate(selectedLibrary.id)} type="button"><RefreshCw size={16} />{refreshLibrary.isPending ? '已提交…' : '刷新此库'}</button> : null}
      </div>

      <div className={itemId ? 'library-browser has-detail' : 'library-browser'}>
        <div className="library-results">
          <div className="library-results-heading">
            <div><strong>{submittedQuery ? `“${submittedQuery}”的结果` : selectedLibrary?.name ?? '媒体内容'}</strong><span>{total} 项</span></div>
            {!submittedQuery && total > pageSize ? <div className="library-pagination"><IconButton label="上一页" disabled={page === 0} onClick={() => changePage(Math.max(0, page - 1))} subtle><ChevronLeft size={17} /></IconButton><span>{page + 1} / {Math.max(1, Math.ceil(total / pageSize))}</span><IconButton label="下一页" disabled={(page + 1) * pageSize >= total} onClick={() => changePage(page + 1)} subtle><ChevronRight size={17} /></IconButton></div> : null}
          </div>
          {result.isLoading ? <div className="result-loading"><div /><div /><div /></div> : null}
          {result.isError ? <div className="inline-error"><CircleAlert size={18} /><div><strong>媒体内容读取失败</strong><span>{result.error.message}</span></div><button onClick={() => void result.refetch()} type="button">重试</button></div> : null}
          {!result.isLoading && !result.isError && items.length === 0 ? <div className="empty-state"><BookOpen size={28} /><span>{submittedQuery ? '没有匹配的媒体' : '此媒体库暂无可浏览内容'}</span></div> : null}
          <div className="library-item-list">
            {items.map((item) => <LibraryItemButton item={item} key={item.id} selected={item.id === itemId} onSelect={selectItem} />)}
          </div>
        </div>

        <aside className="library-detail" aria-label="媒体详情">
          {itemId ? <div className="library-detail-back"><IconButton label="返回媒体列表" onClick={closeItem} subtle><ChevronLeft size={17} /></IconButton><span>返回媒体列表</span></div> : null}
          {!itemId ? <div className="library-detail-empty"><Film size={28} /><strong>选择一个媒体</strong><span>查看简介、年份和播放信息。</span></div> : null}
          {detail.isLoading ? <div className="status-loading">正在读取媒体详情…</div> : null}
          {detail.isError ? <div className="inline-error"><CircleAlert size={18} /><div><strong>详情读取失败</strong><span>{detail.error.message}</span></div><button onClick={() => void detail.refetch()} type="button">重试</button></div> : null}
          {detail.data ? <LibraryItemDetail item={detail.data} onRefresh={(id) => refreshItem.mutate(id)} refreshing={refreshItem.isPending} /> : null}
        </aside>
      </div>
    </section>
  )
}

function LibraryItemDetail({ item, onRefresh, refreshing }: { item: EmbyItemDetail; onRefresh: (id: string) => void; refreshing: boolean }) {
  const originalTitle = visibleOriginalTitle(item)
  const genres = localizedGenres(item.genres ?? [])

  return <>
    <div className="library-detail-heading"><div><h2>{item.name}</h2></div><span className="media-type-chip">{mediaTypeLabel(item.type)}</span></div>
    <dl className="library-facts">
      <div><dt>年份</dt><dd>{item.year || '未知'}</dd></div>
      <div><dt>时长</dt><dd>{item.runtimeMinutes ? `${item.runtimeMinutes} 分钟` : '未提供'}</dd></div>
      <div><dt>评分</dt><dd>{item.communityRating ? item.communityRating.toFixed(1) : '未提供'}</dd></div>
    </dl>
    {genres.length ? <div className="library-genres">{genres.map((genre) => <span key={genre}>{genre}</span>)}</div> : null}
    <p className="library-overview">{item.overview || '暂未提供简介。'}</p>
    <details className="library-more">
      <summary>更多信息</summary>
      <dl>
        {originalTitle ? <div><dt>原名</dt><dd>{originalTitle}</dd></div> : null}
        <div><dt>TMDB 编号</dt><dd>{item.providerIds?.Tmdb || '未绑定'}</dd></div>
        <div><dt>媒体源</dt><dd>{item.type === 'Series' ? '由分集提供' : `${item.mediaSourceCount} 个`}</dd></div>
      </dl>
    </details>
    <div className="library-detail-actions">
      <button className="secondary-command" disabled={refreshing} onClick={() => onRefresh(item.id)} type="button"><RefreshCw size={16} />{refreshing ? '已提交…' : '刷新元数据'}</button>
      <a className="primary-action compact" href={item.externalUrl} rel="noreferrer" target="_blank"><ExternalLink size={16} />在 Emby 中打开</a>
    </div>
  </>
}

function LibraryItemButton({ item, selected, onSelect }: { item: EmbyItem; selected: boolean; onSelect: (id: string) => void }) {
  return <button aria-pressed={selected} className={selected ? 'library-item selected' : 'library-item'} onClick={() => onSelect(item.id)} type="button"><span className="library-item-icon"><Film size={18} /></span><span><strong>{item.name}</strong><small>{mediaTypeLabel(item.type)}{item.year ? ` · ${item.year}` : ''}</small></span><ChevronRight size={17} /></button>
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
