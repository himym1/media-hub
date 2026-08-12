import { useState, type FormEvent } from 'react'
import { useQuery } from '@tanstack/react-query'
import { CircleAlert, Film, FolderOpen, Search } from 'lucide-react'
import { getEmbyLibraries, searchEmbyItems } from '../../shared/api/mediaHub'

export function LibraryView() {
  const [query, setQuery] = useState('')
  const [submittedQuery, setSubmittedQuery] = useState('')
  const libraries = useQuery({ queryKey: ['emby-libraries'], queryFn: getEmbyLibraries })
  const items = useQuery({
    queryKey: ['emby-items', submittedQuery],
    queryFn: () => searchEmbyItems(submittedQuery, 50),
    enabled: Boolean(submittedQuery),
  })

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const next = query.trim()
    if (next) setSubmittedQuery(next)
  }

  return (
    <section className="workspace-view">
      <div className="workspace-heading"><div><p className="eyebrow">EMBY LIBRARY</p><h1>媒体库</h1></div><span className="view-count">{libraries.data?.libraries.length ?? 0} 个库</span></div>

      <form className="compact-search" onSubmit={submit}>
        <Search aria-hidden="true" size={19} />
        <label className="sr-only" htmlFor="emby-search">搜索 Emby 媒体库</label>
        <input id="emby-search" maxLength={120} onChange={(event) => setQuery(event.target.value)} placeholder="搜索已入库电影或剧集" value={query} />
        <button disabled={!query.trim() || items.isFetching} type="submit">{items.isFetching ? '搜索中' : '搜索'}</button>
      </form>

      {libraries.isError ? <div className="inline-error"><CircleAlert size={18} /><div><strong>Emby 暂时不可用</strong><span>{libraries.error.message}</span></div><button onClick={() => void libraries.refetch()} type="button">重试</button></div> : null}
      <div className="library-strip" aria-label="Emby 媒体库">
        {libraries.data?.libraries.map((library) => <div className="library-item" key={library.id}><FolderOpen size={18} /><div><strong>{library.name}</strong><span>{library.collectionType ?? '媒体'}</span></div></div>)}
      </div>

      <div className="section-heading library-results-heading"><div><p className="eyebrow">LIBRARY RESULTS</p><h2>{submittedQuery || '搜索结果'}<span>{items.data?.total ?? 0}</span></h2></div></div>
      {items.isError ? <div className="inline-error"><CircleAlert size={18} /><div><strong>搜索失败</strong><span>{items.error.message}</span></div><button onClick={() => void items.refetch()} type="button">重试</button></div> : null}
      {!submittedQuery ? <div className="empty-state"><Search size={26} /><span>搜索 Emby 中已入库的媒体</span></div> : null}
      {submittedQuery && !items.isFetching && items.data?.items.length === 0 ? <div className="empty-state"><Film size={26} /><span>没有找到已入库项目</span></div> : null}
      <div className="media-list">
        {items.data?.items.map((item) => <article className="media-row" key={item.id}><span className="media-icon"><Film size={18} /></span><div><strong>{item.name}</strong><span>{item.type}{item.year ? ` · ${item.year}` : ''}</span></div></article>)}
      </div>
    </section>
  )
}
