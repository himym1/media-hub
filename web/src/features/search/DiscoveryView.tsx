import { useEffect, useRef, useState, type FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { BellPlus, CircleAlert, Film, FolderInput, RefreshCw, Search, X } from 'lucide-react'
import {
  createTransfer,
  getRecommendations,
  getTrending,
  searchMedia,
  type Candidate,
  type Integration,
} from '../../shared/api/mediaHub'
import { IconButton } from '../../shared/ui/IconButton'

function formatSize(bytes: number) {
  if (bytes <= 0) return '大小未知'
  const gib = bytes / 1024 / 1024 / 1024
  return gib >= 1024 ? `${(gib / 1024).toFixed(1)} TB` : `${gib.toFixed(1)} GB`
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
  const [query, setQuery] = useState('')
  const [submittedQuery, setSubmittedQuery] = useState('')
  const [selected, setSelected] = useState<Candidate | null>(null)
  const sourceIntegration = integrations.find((item) => item.id === 'sources')
  const healthyCount = integrations.filter((item) => item.status === 'healthy').length

  const search = useQuery({
    queryKey: ['search', submittedQuery],
    queryFn: () => searchMedia(submittedQuery),
    enabled: Boolean(submittedQuery),
  })
  const trending = useQuery({ queryKey: ['tmdb-trending'], queryFn: () => getTrending('all', 10), retry: false })
  const transfer = useMutation({
    mutationFn: (candidate: Candidate) => {
      if (!candidate.transferToken) throw new Error('当前资源不能转存')
      return createTransfer(candidate.transferToken, crypto.randomUUID())
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['transfers'] })
      onTransferCreated()
    },
  })

  useEffect(() => {
    const focusSearch = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault()
        inputRef.current?.focus()
      }
    }
    window.addEventListener('keydown', focusSearch)
    return () => window.removeEventListener('keydown', focusSearch)
  }, [])

  const recommendations = useQuery({
    queryKey: ['tmdb-recommendations', selected?.mediaType, selected?.tmdbId],
    queryFn: () => getRecommendations(selected!.mediaType, selected!.tmdbId!, 6),
    enabled: Boolean(selected?.tmdbId),
    retry: false,
  })

  const handleSearch = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const next = query.trim()
    if (!next) return
    setSubmittedQuery(next)
    setSelected(null)
  }
  const searchDiscoveryItem = (title: string) => {
    setQuery(title)
    setSubmittedQuery(title)
    setSelected(null)
  }
  const handleTransfer = (candidate: Candidate) => {
    setSelected(candidate)
    transfer.mutate(candidate)
  }

  return (
    <section className="discovery-view">
      <header className="view-header discovery-header">
        <div><p className="eyebrow">DISCOVER</p><h1>发现</h1><p>搜索资源、核对版本并加入自动转存流程。</p></div>
        <button className="health-summary" disabled={integrationsLoading} onClick={onRefreshIntegrations} type="button"><span className={healthyCount > 0 ? 'healthy' : ''} /><strong>{integrationsLoading ? '检查中…' : `${healthyCount}/${integrations.length} 服务在线`}</strong><RefreshCw size={15} /></button>
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
            onChange={(event) => setQuery(event.target.value)}
            placeholder="搜索电影或电视剧…"
            ref={inputRef}
            value={query}
          />
          <button disabled={!query.trim() || search.isFetching} type="submit">{search.isFetching ? '搜索中…' : '搜索'}</button>
        </form>
        <div className="search-meta"><span>{sourceIntegration?.detail ?? '资源源尚未配置'}</span>{submittedQuery ? <><span>·</span><span>{search.data?.partial ? '部分结果' : '搜索完成'}</span></> : null}</div>
      </section>

      {!submittedQuery && trending.data?.items.length ? (
        <section className="trending-band" aria-label="本周热门">
          <div className="section-heading"><div><p className="eyebrow">TRENDING</p><h2>本周热门</h2></div></div>
          <div className="trending-list">
            {trending.data.items.map((item) => (
              <button key={`${item.mediaType}-${item.tmdbId}`} onClick={() => searchDiscoveryItem(item.title)} type="button">
                <span className="trending-poster">{item.posterUrl ? <img alt="" height="210" loading="lazy" src={item.posterUrl} width="140" /> : <Film aria-hidden="true" size={20} />}</span>
                <span><strong>{item.title}</strong><small>{item.year || '年份未知'} · {item.mediaType === 'movie' ? '电影' : '剧集'}</small></span>
              </button>
            ))}
          </div>
        </section>
      ) : null}

      {search.data?.sourceErrors.length ? <div className="source-warning" role="status"><CircleAlert size={16} /><span>{search.data.sourceErrors.map((item) => item.message).join(' · ')}</span></div> : null}
      {transfer.isError ? <div className="source-warning error" role="alert"><CircleAlert size={16} /><span>{transfer.error.message}</span></div> : null}

      {submittedQuery ? (
        <div className={selected ? 'content-grid has-detail' : 'content-grid'}>
          <section className="results-column">
            <div className="section-heading"><div><p className="eyebrow">RESULTS</p><h2>{search.data?.query ?? submittedQuery}<span>{search.data?.results.length ?? 0}</span></h2></div></div>
            {search.isLoading ? <div className="result-loading"><div /><div /><div /></div> : null}
            {search.isError ? <div className="inline-error"><CircleAlert size={18} /><div><strong>搜索暂时不可用</strong><span>{search.error.message}</span></div><button onClick={() => void search.refetch()} type="button">重试</button></div> : null}
            {!search.isLoading && !search.isError && search.data?.results.length === 0 ? <div className="empty-state"><Film size={26} /><span>没有找到匹配资源</span></div> : null}
            {!search.isLoading && !search.isError ? search.data?.results.map((candidate) => {
              const transferring = transfer.isPending && transfer.variables.id === candidate.id
              const available = candidate.transferState === 'available' && Boolean(candidate.transferToken)
              const subscribable = candidate.transferState !== 'identity_required' && Boolean(candidate.tmdbId)
              const availabilityLabel = available ? '可转存' : candidate.transferState === 'identity_required' ? '身份待确认' : '不可转存'
              return (
                <article className={selected?.id === candidate.id ? 'result-row selected' : 'result-row'} key={candidate.id}>
                  <div className="poster-wrap">{candidate.posterUrl ? <img alt={`${candidate.title} 海报`} height="144" loading="lazy" src={candidate.posterUrl} width="96" /> : <Film aria-hidden="true" size={24} />}<span>{candidate.mediaType === 'movie' ? '电影' : '剧集'}</span></div>
                  <button aria-pressed={selected?.id === candidate.id} className="result-main result-select" onClick={() => setSelected(candidate)} type="button">
                    <span className="result-title-row"><span className="result-title">{candidate.title}</span><span className="year">{candidate.year}</span></span>
                    <span className="source-line"><span className="source-badge">{candidate.provider ?? candidate.source}</span>{episodeLabel(candidate) ? <span>{episodeLabel(candidate)}</span> : null}<span>{candidate.release.resolution}</span><span>{candidate.release.videoCodec}</span>{candidate.release.dynamicRange ? <span>{candidate.release.dynamicRange}</span> : null}</span>
                    <span className="result-facts"><span>{candidate.release.audio ?? '音轨未知'}</span><span>·</span><span>{formatSize(candidate.release.sizeBytes)}</span></span>
                  </button>
                  <div className="result-action"><span className={`availability ${available ? 'available' : 'unavailable'}`}><span />{availabilityLabel}</span><div className="result-commands"><IconButton disabled={!subscribable} label="创建订阅" onClick={() => onSubscribe(candidate)}><BellPlus size={15} /></IconButton><button disabled={!available || transferring} onClick={() => handleTransfer(candidate)} type="button">{transferring ? '提交中…' : '转存'}<FolderInput size={15} /></button></div></div>
                </article>
              )
            }) : null}
          </section>

          {selected ? <aside className="detail-panel" aria-label="资源详情">
            <div className="detail-header"><div><p className="eyebrow">VERSION</p><h2>{selected.title}</h2></div><IconButton label="关闭详情" onClick={() => setSelected(null)} subtle><X size={17} /></IconButton></div>
            <div className="detail-poster">{selected.posterUrl ? <img alt={`${selected.title} 海报`} height="270" src={selected.posterUrl} width="360" /> : <div className="poster-placeholder"><Film size={34} /></div>}<div className="poster-overlay"><span>{selected.provider ?? selected.source}</span><strong>{selected.release.resolution}</strong></div></div>
            <div className="detail-facts"><div><span>视频</span><strong>{selected.release.videoCodec}{selected.release.dynamicRange ? ` · ${selected.release.dynamicRange}` : ''}</strong></div><div><span>音频</span><strong>{selected.release.audio ?? '未知'}</strong></div><div><span>体积</span><strong>{formatSize(selected.release.sizeBytes)}</strong></div><div><span>目标</span><strong>{selected.mediaType === 'movie' ? '115 / 电影' : '115 / 电视剧'}</strong></div></div>
            {recommendations.data?.items.length ? <div className="recommendation-links"><span>相似内容</span>{recommendations.data.items.slice(0, 4).map((item) => <button key={`${item.mediaType}-${item.tmdbId}`} onClick={() => searchDiscoveryItem(item.title)} type="button">{item.title}</button>)}</div> : null}
            <div className="detail-actions"><button className="secondary-command" disabled={!selected.tmdbId || selected.transferState === 'identity_required'} onClick={() => onSubscribe(selected)} type="button"><BellPlus size={16} />订阅</button><button className="primary-action" disabled={!selected.transferToken || transfer.isPending} onClick={() => handleTransfer(selected)} type="button"><FolderInput size={17} />{transfer.isPending ? '正在创建任务…' : selected.transferToken ? '加入转存队列' : selected.transferState === 'identity_required' ? '身份待确认' : '工作流不可用'}</button></div>
          </aside> : null}
        </div>
      ) : null}
    </section>
  )
}

function episodeLabel(candidate: Candidate) {
  if (!candidate.season) return ''
  if (!candidate.episodeStart) return `S${candidate.season}`
  return candidate.episodeStart === candidate.episodeEnd
    ? `S${candidate.season}E${candidate.episodeStart}`
    : `S${candidate.season}E${candidate.episodeStart}-${candidate.episodeEnd}`
}
