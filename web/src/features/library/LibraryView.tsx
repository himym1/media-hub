import { useEffect, useMemo, useState, type FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { BookOpen, Captions, ChevronLeft, ChevronRight, CircleAlert, Film, Play, RefreshCw, Search, Trash2, X } from 'lucide-react'
import {
  deleteEmbyItem,
  downloadEmbyRemoteSubtitle,
  embyPrimaryImageURL,
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
import { commitUrl } from '../../shared/navigation/urlState'
import { IconButton } from '../../shared/ui/IconButton'
import { LibraryEpisodes } from './LibraryEpisodes'
import { LibraryPlayer } from './LibraryPlayer'
import { episodeLabel, playbackActionLabel, playbackStatus } from './libraryPlayback'

const pageSize = 24

type LibraryScope = 'mine' | 'shared'

function locationValue(name: string) {
  return new URLSearchParams(window.location.search).get(name)
}

function isSharedEmbyId(id: string | null | undefined) {
  return Boolean(id?.startsWith('r_'))
}

function libraryDisplayName(name: string) {
  return name.replace(/^共享\//, '')
}

export function LibraryView() {
  const queryClient = useQueryClient()
  const [libraryId, setLibraryId] = useState<string | null>(() => locationValue('library'))
  const [itemId, setItemId] = useState<string | null>(() => locationValue('media'))
  const [playId, setPlayId] = useState<string | null>(() => locationValue('play'))
  const [page, setPage] = useState(0)
  const [queryText, setQueryText] = useState('')
  const [submittedQuery, setSubmittedQuery] = useState('')
  const [scope, setScope] = useState<LibraryScope>(() => (isSharedEmbyId(locationValue('library')) ? 'shared' : 'mine'))

  const libraries = useQuery({ queryKey: ['emby-libraries'], queryFn: getEmbyLibraries })
  const mineLibraries = useMemo(
    () => (libraries.data?.libraries ?? []).filter((library) => !isSharedEmbyId(library.id)),
    [libraries.data?.libraries],
  )
  const sharedLibraries = useMemo(
    () => (libraries.data?.libraries ?? []).filter((library) => isSharedEmbyId(library.id)),
    [libraries.data?.libraries],
  )
  const allLibraries = libraries.data?.libraries ?? []
  const scopeLibraries = scope === 'shared' ? sharedLibraries : mineLibraries
  const hasShared = sharedLibraries.length > 0

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
    if (!libraries.data) return
    if (!hasShared && scope === 'shared') {
      setScope('mine')
      return
    }
    if (libraryId && scopeLibraries.some((library) => library.id === libraryId)) return
    const fallback = scopeLibraries[0]?.id ?? null
    setLibraryId(fallback)
    setItemId(null)
    setPlayId(null)
    setPage(0)
    commitUrl({ library: fallback, media: null, play: null }, 'replace')
  }, [libraries.data, libraryId, scope, scopeLibraries, hasShared])

  const selectScope = (next: LibraryScope) => {
    if (next === scope) return
    setScope(next)
    setSubmittedQuery('')
    setQueryText('')
    setItemId(null)
    setPlayId(null)
    setPage(0)
    const fallback = (next === 'shared' ? sharedLibraries : mineLibraries)[0]?.id ?? null
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
  const startPlay = (target: Pick<EmbyEpisode, 'id' | 'name' | 'externalUrl'>) => {
    setPlayId(target.id)
    commitUrl({ play: target.id })
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
    commitUrl({ media: null, play: null })
  }
  const clearSearch = () => {
    setQueryText('')
    setSubmittedQuery('')
    setItemId(null)
    setPlayId(null)
    commitUrl({ media: null, play: null })
  }
  const changePage = (nextPage: number) => {
    setPage(nextPage)
    setItemId(null)
    setPlayId(null)
    commitUrl({ media: null, play: null })
  }

  const result = submittedQuery ? search : libraryItems
  const items = useMemo(() => {
    const list = result.data?.items ?? []
    if (!submittedQuery) return list
    return list.filter((item) => (scope === 'shared' ? isSharedEmbyId(item.id) : !isSharedEmbyId(item.id)))
  }, [result.data?.items, submittedQuery, scope])
  const total = submittedQuery ? items.length : (result.data?.total ?? 0)
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
    <section className="library-page">
      <header className="view-header compact-view-header">
        <div><h1>媒体库</h1><p>浏览已入库内容，查看媒体信息并按需刷新元数据。</p></div>
        <IconButton label="刷新媒体库列表" onClick={() => void libraries.refetch()} subtle><RefreshCw size={17} /></IconButton>
      </header>

      {libraries.isError ? <div className="inline-error"><CircleAlert size={18} /><div><strong>媒体库读取失败</strong><span>{libraries.error.message}</span></div><button onClick={() => void libraries.refetch()} type="button">重试</button></div> : null}
      {mutationError ? <div className="source-warning error" role="alert"><CircleAlert size={16} /><span>{mutationError.message}</span></div> : null}

      {hasShared ? (
        <div className="library-scope" role="tablist" aria-label="媒体库来源">
          <button aria-selected={scope === 'mine'} className={scope === 'mine' ? 'library-scope-button selected' : 'library-scope-button'} onClick={() => selectScope('mine')} role="tab" type="button">我的库</button>
          <button aria-selected={scope === 'shared'} className={scope === 'shared' ? 'library-scope-button selected' : 'library-scope-button'} onClick={() => selectScope('shared')} role="tab" type="button">共享库</button>
        </div>
      ) : null}

      <nav className="library-selector" aria-label={scope === 'shared' ? '共享 Emby 媒体库' : '我的 Emby 媒体库'}>
        {scopeLibraries.map((library) => {
          const typeLabel = libraryCollectionLabel(library.collectionType)
          const name = libraryDisplayName(library.name)
          return (
            <button aria-pressed={library.id === libraryId} className={library.id === libraryId ? 'library-button selected' : 'library-button'} key={library.id} onClick={() => selectLibrary(library.id)} type="button">
              <span><strong>{name}</strong>{typeLabel.toLocaleLowerCase() !== name.trim().toLocaleLowerCase() ? <small>{typeLabel}</small> : null}</span>
            </button>
          )
        })}
      </nav>

      <div className="library-toolbar">
        <form className="library-search" onSubmit={submitSearch}>
          <Search size={19} aria-hidden="true" />
          <label className="sr-only" htmlFor="library-query">搜索 Emby 媒体</label>
          <input
            autoComplete="off"
            id="library-query"
            maxLength={120}
            name="library-query"
            onChange={(event) => setQueryText(event.target.value)}
            placeholder={scope === 'shared' ? '搜索共享库…' : '搜索电影或剧集…'}
            type="search"
            value={queryText}
          />
          {queryText ? (
            <button aria-label="清空搜索内容" className="search-clear-button" onClick={clearSearch} type="button">
              <X aria-hidden="true" size={16} />
            </button>
          ) : null}
          <button disabled={!queryText.trim()} type="submit">搜索</button>
        </form>
        {selectedLibrary && !submittedQuery && !isSharedEmbyId(selectedLibrary.id) ? <button className="secondary-command" disabled={refreshLibrary.isPending} onClick={() => refreshLibrary.mutate(selectedLibrary.id)} type="button"><RefreshCw size={16} />{refreshLibrary.isPending ? '已提交…' : '刷新此库'}</button> : null}
      </div>

      <div className={itemId ? 'library-browser has-detail' : 'library-browser'}>
        <div className="library-results">
          <div className="library-results-heading">
            <div>
              <strong>{submittedQuery ? `“${submittedQuery}”的结果` : libraryDisplayName(selectedLibrary?.name ?? '媒体内容')}</strong>
              <span>{total} 项</span>
            </div>
            {!submittedQuery && total > pageSize ? (
              <div className="library-pagination">
                <IconButton label="上一页" disabled={page === 0} onClick={() => changePage(Math.max(0, page - 1))} subtle><ChevronLeft size={17} /></IconButton>
                <span>{page + 1} / {Math.max(1, Math.ceil(total / pageSize))}</span>
                <IconButton label="下一页" disabled={(page + 1) * pageSize >= total} onClick={() => changePage(page + 1)} subtle><ChevronRight size={17} /></IconButton>
              </div>
            ) : null}
          </div>
          {result.isLoading && items.length === 0 ? <div className="result-loading"><div /><div /><div /></div> : null}
          {result.isError ? <div className="inline-error"><CircleAlert size={18} /><div><strong>媒体内容读取失败</strong><span>{result.error.message}</span></div><button onClick={() => void result.refetch()} type="button">重试</button></div> : null}
          {!result.isLoading && !result.isError && items.length === 0 ? <div className="empty-state"><BookOpen size={28} /><span>{submittedQuery ? '没有匹配的媒体' : '此媒体库暂无可浏览内容'}</span></div> : null}
          <div className="library-poster-grid">
            {items.map((item) => <LibraryPosterCard item={item} key={item.id} selected={item.id === itemId} onSelect={selectItem} />)}
          </div>
        </div>

        <aside className="library-detail" aria-label="媒体详情">
          {itemId ? <div className="library-detail-back"><IconButton label="返回媒体列表" onClick={closeItem} subtle><ChevronLeft size={17} /></IconButton><span>返回媒体列表</span></div> : null}
          {!itemId ? <div className="library-detail-empty"><Film size={28} /><strong>选择一个媒体</strong><span>查看简介、年份和播放信息。</span></div> : null}
          {detail.isLoading && !detail.data ? <div className="status-loading">正在读取媒体详情…</div> : null}
          {detail.isError ? <div className="inline-error"><CircleAlert size={18} /><div><strong>详情读取失败</strong><span>{detail.error.message}</span></div><button onClick={() => void detail.refetch()} type="button">重试</button></div> : null}
          {detail.data ? <LibraryItemDetail key={detail.data.id} item={detail.data} onDeleted={closeItem} onPlay={startPlay} onRefresh={(id) => refreshItem.mutate(id)} refreshing={refreshItem.isPending} shared={isSharedEmbyId(detail.data.id)} /> : null}
        </aside>
      </div>
      {playTarget ? (
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
  const status = item.played || (item.playbackPositionMs ?? 0) >= 30_000 ? playbackStatus(item) : ''
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
          <span className="library-poster-fallback" aria-hidden="true"><Film size={28} /></span>
        )}
        {status ? <span className={item.played ? 'library-poster-badge played' : 'library-poster-badge'}>{status}</span> : null}
      </span>
      <span className="library-poster-meta">
        <strong>{item.name}</strong>
        <small>{mediaTypeLabel(item.type)}{item.year ? ` · ${item.year}` : ''}</small>
      </span>
    </button>
  )
}

function LibraryItemDetail({ item, onDeleted, onPlay, onRefresh, refreshing, shared }: {
  item: EmbyItemDetail
  onDeleted: () => void
  onPlay: (target: Pick<EmbyEpisode, 'id' | 'name' | 'externalUrl'>) => void
  onRefresh: (id: string) => void
  refreshing: boolean
  shared: boolean
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
  })
  const preview = previewDelete.data
  const busy = previewDelete.isPending || confirmDelete.isPending || refreshing || searchSubtitles.isPending || downloadSubtitle.isPending
  const deleteError = previewDelete.error ?? confirmDelete.error
  const subtitleError = searchSubtitles.error ?? downloadSubtitle.error
  const canSearchSubtitles = item.type !== 'Series'

  return <>
    <div className="library-detail-hero">
      <div className="library-detail-poster">
        {!posterFailed ? (
          <img alt="" decoding="async" height={300} onError={() => setPosterFailed(true)} src={embyPrimaryImageURL(item.id)} width={200} />
        ) : (
          <span className="library-poster-fallback" aria-hidden="true"><Film size={36} /></span>
        )}
      </div>
      <div className="library-detail-hero-copy">
        <div className="library-detail-heading">
          <div><h2>{item.name}</h2></div>
          <span className="media-type-chip">{mediaTypeLabel(item.type)}</span>
        </div>
        <dl className="library-facts compact">
          <div><dt>年份</dt><dd>{item.year || '未知'}</dd></div>
          <div><dt>时长</dt><dd>{item.runtimeMinutes ? `${item.runtimeMinutes} 分钟` : '未提供'}</dd></div>
          <div><dt>评分</dt><dd>{item.communityRating ? item.communityRating.toFixed(1) : '未提供'}</dd></div>
          <div><dt>进度</dt><dd>{playbackStatus(item)}</dd></div>
        </dl>
        {genres.length ? <div className="library-genres">{genres.map((genre) => <span key={genre}>{genre}</span>)}</div> : null}
      </div>
    </div>
    <p className="library-overview">{item.overview || '暂未提供简介。'}</p>
    <details className="library-more">
      <summary>更多信息</summary>
      <dl>
        {originalTitle ? <div><dt>原名</dt><dd>{originalTitle}</dd></div> : null}
        <div><dt>TMDB 编号</dt><dd>{item.providerIds?.Tmdb || '未绑定'}</dd></div>
        <div><dt>媒体源</dt><dd>{item.type === 'Series' ? '由分集提供' : `${item.mediaSourceCount} 个`}</dd></div>
      </dl>
    </details>
    {shared ? <p className="library-shared-note">共享库只读；播放由播放设备直连 Emby（通常经代理）。</p> : null}
    <div className="library-detail-actions">
      {item.type === 'Series' ? null : (
        <button className="primary-action" disabled={busy} onClick={() => onPlay(item)} type="button">
          <Play size={16} />
          {playbackActionLabel(item)}
        </button>
      )}
      {shared ? null : <button className="secondary-command" disabled={busy} onClick={() => onRefresh(item.id)} type="button"><RefreshCw size={16} />{refreshing ? '已提交…' : '刷新元数据'}</button>}
      {shared ? null : canSearchSubtitles ? (
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
      {shared || preview ? null : <button className="danger-button" disabled={busy} onClick={() => previewDelete.mutate()} type="button"><Trash2 size={16} />{previewDelete.isPending ? '正在读取删除预览…' : '从 Emby 删除'}</button>}
    </div>
    {item.type === 'Series' ? <LibraryEpisodes onPlay={onPlay} seriesId={item.id} seriesTitle={item.name} /> : null}
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
  return `将从 Emby 删除「${preview.name}」${versions}${series}。NAS 上约 ${preview.fileCount} 个 STRM 和同名字幕会被清掉，115 网盘文件不会删除。`
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
