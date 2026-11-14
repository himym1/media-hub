import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Captions, CircleAlert, ExternalLink, Film, RefreshCw, Star, Trash2, X } from 'lucide-react'
import {
  deleteEmbyItem,
  embyBackdropImageURL,
  embyPrimaryImageURL,
  fetchLocalSubtitle,
  previewEmbyItemDelete,
  searchEmbyRemoteSubtitles,
  downloadEmbyRemoteSubtitle,
  type EmbyDeletePreview,
  type EmbyEpisode,
  type EmbyItemDetail,
  type EmbyMediaTechSpecs,
  type EmbyRemoteSubtitle,
} from '../../shared/api/mediaHub'
import { IconButton } from '../../shared/ui/IconButton'
import { LibraryCastGallery } from './LibraryCastGallery'
import { LibraryEpisodes } from './LibraryEpisodes'
import { LibraryWatchAction } from './LibraryWatchAction'
import { mediaTypeLabel, playbackStatus } from './libraryPlayback'
import { buildTechBadges } from './libraryTechSpecs'

function TechSpecsBadges({ specs }: { specs?: EmbyMediaTechSpecs | null }) {
  const badges = buildTechBadges(specs)
  if (badges.length === 0) return null

  return (
    <div aria-label="技术规格" className="tech-specs-strip">
      {badges.map((b) => (
        <span className={b.className} key={b.key}>
          {b.label}
        </span>
      ))}
    </div>
  )
}

export function LibraryItemDetail({
  item,
  inPagePlayback,
  onDeleted,
  onPlay,
  onRefresh,
  onSelectPerson,
  refreshing,
  seriesEpisodes,
  shared,
}: {
  item: EmbyItemDetail
  inPagePlayback: boolean
  onDeleted: () => void
  onPlay: (target: { id: string }) => void
  onRefresh: (id: string) => void
  onSelectPerson?: (name: string) => void
  refreshing: boolean
  seriesEpisodes?: EmbyEpisode[]
  shared: boolean
}) {
  const queryClient = useQueryClient()
  const originalTitle = visibleOriginalTitle(item)
  const genres = localizedGenres(item.genres ?? [])
  const [posterFailed, setPosterFailed] = useState(false)
  const [posterLoaded, setPosterLoaded] = useState(false)
  const [backdropFailed, setBackdropFailed] = useState(false)
  const [subtitleResults, setSubtitleResults] = useState<EmbyRemoteSubtitle[] | null>(null)
  const effectiveSpecs = item.techSpecs ?? seriesEpisodes?.find((ep) => ep.techSpecs)?.techSpecs
  const backdropUrl = !backdropFailed ? embyBackdropImageURL(item.id) : embyPrimaryImageURL(item.id)
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
    enabled: !shared && item.type !== 'Series',
    retry: false,
    staleTime: 60_000,
  })
  const preview = previewDelete.data
  const busy = previewDelete.isPending || confirmDelete.isPending || refreshing || searchSubtitles.isPending || downloadSubtitle.isPending
  const deleteError = previewDelete.error ?? confirmDelete.error
  const subtitleError = searchSubtitles.error ?? downloadSubtitle.error
  const canSearchSubtitles = item.type !== 'Series'

  return <>
    <div aria-hidden="true" className="detail-hero-backdrop-wrap">
      <div
        className="detail-hero-backdrop"
        style={{ backgroundImage: `url(${backdropUrl})` }}
      />
      {!backdropFailed ? (
        <img
          alt=""
          aria-hidden="true"
          className="detail-backdrop-tester"
          onError={() => setBackdropFailed(true)}
          src={embyBackdropImageURL(item.id)}
        />
      ) : null}
      <div className="detail-backdrop-gradient-bottom" />
      <div className="detail-backdrop-gradient-left" />
    </div>
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
            <img
              alt=""
              className={`detail-poster-foreground fade-in-image ${posterLoaded ? 'loaded' : ''}`}
              decoding="async"
              height={360}
              onError={() => setPosterFailed(true)}
              onLoad={() => setPosterLoaded(true)}
              src={embyPrimaryImageURL(item.id)}
              width={240}
            />
          </>
        ) : (
          <span aria-hidden="true" className="library-poster-fallback"><Film size={36} /></span>
        )}
      </div>
      <div className="library-detail-hero-copy">
        <div className="library-detail-heading">
          <div>
            <h2>{item.name}</h2>
          </div>
          <div className="library-heading-tags">
            {item.communityRating ? (
              <span className="rating-pill-gold" title={`TMDB 评分: ${item.communityRating.toFixed(1)}`}>
                <Star className="rating-star" size={13} />
                <span className="rating-score">{item.communityRating.toFixed(1)}</span>
              </span>
            ) : null}
            <span className="media-type-chip">{mediaTypeLabel(item.type)}</span>
          </div>
        </div>
        <TechSpecsBadges specs={effectiveSpecs} />
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
          {shared || item.type === 'Series' ? null : (
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
    <LibraryCastGallery onSelectPerson={onSelectPerson} people={item.people} />
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
    {shared ? <p className="library-shared-note">共享库只读；播放由播放设备直连远程 Emby，不经 Media Hub 转发视频。</p> : null}
    <div className="library-detail-actions library-detail-actions-secondary">
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
      ) : shared ? null : (
        <span className="library-subtitle-hint">剧集请在 Emby 分集条目上搜索字幕</span>
      )}
      {shared || preview ? null : <button className="danger-button" disabled={busy} onClick={() => previewDelete.mutate()} type="button"><Trash2 size={16} />{previewDelete.isPending ? '正在读取删除预览…' : '从 Emby 删除'}</button>}
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
