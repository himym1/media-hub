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
        <div><p className="eyebrow">EMBY</p><h1>媒体库</h1><p>浏览已入库内容，查看媒体身份并按需请求 Emby 刷新。</p></div>
        <IconButton label="刷新媒体库列表" onClick={() => void libraries.refetch()} subtle><RefreshCw size={17} /></IconButton>
      </header>

      {libraries.isError ? <div className="inline-error"><CircleAlert size={18} /><div><strong>媒体库读取失败</strong><span>{libraries.error.message}</span></div><button onClick={() => void libraries.refetch()} type="button">重试</button></div> : null}
      {mutationError ? <div className="source-warning error" role="alert"><CircleAlert size={16} /><span>{mutationError.message}</span></div> : null}

      <nav className="library-selector" aria-label="Emby 媒体库">
        {libraries.data?.libraries.map((library) => (
          <button aria-pressed={library.id === libraryId} className={library.id === libraryId ? 'library-button selected' : 'library-button'} key={library.id} onClick={() => selectLibrary(library.id)} type="button">
            <FolderOpen size={18} /><span><strong>{library.name}</strong><small>{library.collectionType || '媒体库'}</small></span>
          </button>
        ))}
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
          {!itemId ? <div className="library-detail-empty"><Film size={28} /><strong>选择一个媒体</strong><span>查看身份、简介和 Emby 状态。</span></div> : null}
          {detail.isLoading ? <div className="status-loading">正在读取媒体详情…</div> : null}
          {detail.isError ? <div className="inline-error"><CircleAlert size={18} /><div><strong>详情读取失败</strong><span>{detail.error.message}</span></div><button onClick={() => void detail.refetch()} type="button">重试</button></div> : null}
          {detail.data ? <>
            <div className="library-detail-heading"><div><p className="eyebrow">DETAIL</p><h2>{detail.data.name}</h2>{detail.data.originalTitle ? <span>{detail.data.originalTitle}</span> : null}</div><span className="media-type-chip">{detail.data.type === 'Movie' ? '电影' : '剧集'}</span></div>
            <dl className="library-facts">
              <div><dt>年份</dt><dd>{detail.data.year || '未知'}</dd></div>
              <div><dt>TMDB</dt><dd>{detail.data.providerIds?.Tmdb || '未绑定'}</dd></div>
              <div><dt>时长</dt><dd>{detail.data.runtimeMinutes ? `${detail.data.runtimeMinutes} 分钟` : '未提供'}</dd></div>
              <div><dt>评分</dt><dd>{detail.data.communityRating ? detail.data.communityRating.toFixed(1) : '未提供'}</dd></div>
            </dl>
            {detail.data.genres?.length ? <div className="library-genres">{detail.data.genres.map((genre) => <span key={genre}>{genre}</span>)}</div> : null}
            <p className="library-overview">{detail.data.overview || 'Emby 暂未提供简介。'}</p>
            <div className="library-media-state"><strong>{detail.data.type === 'Series' ? '剧集播放由分集媒体源提供' : `已关联 ${detail.data.mediaSourceCount} 个媒体源`}</strong><span>Media Hub 不代理或删除媒体文件。</span></div>
            <div className="library-detail-actions">
              <button className="secondary-command" disabled={refreshItem.isPending} onClick={() => refreshItem.mutate(detail.data.id)} type="button"><RefreshCw size={16} />{refreshItem.isPending ? '已提交…' : '刷新元数据'}</button>
              <a className="primary-action compact" href={detail.data.externalUrl} rel="noreferrer" target="_blank"><ExternalLink size={16} />在 Emby 中打开</a>
            </div>
          </> : null}
        </aside>
      </div>
    </section>
  )
}

function LibraryItemButton({ item, selected, onSelect }: { item: EmbyItem; selected: boolean; onSelect: (id: string) => void }) {
  return <button aria-pressed={selected} className={selected ? 'library-item selected' : 'library-item'} onClick={() => onSelect(item.id)} type="button"><span className="library-item-icon"><Film size={18} /></span><span><strong>{item.name}</strong><small>{item.type === 'Movie' ? '电影' : '剧集'}{item.year ? ` · ${item.year}` : ''}{item.providerIds?.Tmdb ? ` · TMDB ${item.providerIds.Tmdb}` : ''}</small></span><ChevronRight size={17} /></button>
}
