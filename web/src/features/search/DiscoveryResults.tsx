import type { ReactNode } from 'react'
import { BellPlus, Download, Film, FolderInput, HardDrive, Volume2 } from 'lucide-react'
import type { Candidate, DiscoveryItem } from '../../shared/api/mediaHub'
import { IconButton } from '../../shared/ui/IconButton'
import {
  dynamicRangeBadgeClass,
  episodeLabel,
  formatCandidateSize,
  isDownloadable,
  pipelineLabel,
  resolutionBadgeClass,
} from './discoverySearch'

export function ResultLane({
  fallbackPoster,
  heading,
  items,
  queuedTransferIds,
  selected,
  transfer,
  onSelect,
  onSubscribe,
  onTransfer,
}: {
  fallbackPoster?: string
  heading: string
  items: Candidate[]
  queuedTransferIds: string[]
  selected: Candidate | null
  transfer: { isPending: boolean; variables?: Candidate }
  onSelect: (candidate: Candidate) => void
  onSubscribe: (candidate: Candidate) => void
  onTransfer: (candidate: Candidate) => void
}) {
  if (items.length === 0) return null
  return (
    <section className="result-lane" aria-label={heading || undefined}>
      {heading ? <h3 className="result-lane-heading">{heading}<span>{items.length}</span></h3> : null}
      {items.map((candidate) => (
        <CandidateRow
          candidate={candidate}
          fallbackPoster={fallbackPoster}
          key={candidate.id}
          queued={queuedTransferIds.includes(candidate.id)}
          selected={selected?.id === candidate.id}
          transferring={transfer.isPending && transfer.variables?.id === candidate.id}
          onSelect={onSelect}
          onSubscribe={onSubscribe}
          onTransfer={onTransfer}
        />
      ))}
    </section>
  )
}

function CandidateRow({
  candidate,
  fallbackPoster,
  queued,
  selected,
  transferring: submitting,
  onSelect,
  onSubscribe,
  onTransfer,
}: {
  candidate: Candidate
  fallbackPoster?: string
  queued: boolean
  selected: boolean
  transferring: boolean
  onSelect: (candidate: Candidate) => void
  onSubscribe: (candidate: Candidate) => void
  onTransfer: (candidate: Candidate) => void
}) {
  const transferring = submitting || queued || candidate.transferState === 'transferring'
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
  const poster = candidate.posterUrl || fallbackPoster
  return (
    <article className={selected ? 'result-row selected' : 'result-row'} data-candidate-id={candidate.id}>
      <div className="poster-wrap">{poster ? <img alt={`${candidate.title} 海报`} height="144" loading="lazy" src={poster} width="96" /> : <Film aria-hidden="true" size={24} />}<span>{candidate.mediaType === 'movie' ? '电影' : '剧集'}</span></div>
      <button aria-pressed={selected} className="result-main result-select" onClick={() => onSelect(candidate)} type="button">
        <span className="result-title-row"><span className="result-title">{candidate.title}</span><span className="year">{candidate.year}</span></span>
        <span className="source-line">
          <span className={`spec-badge ${downloadable ? 'spec-pipeline-pt' : 'spec-pipeline-115'}`}>{pipelineLabel(candidate)}</span>
          <span className="spec-badge spec-source">{candidate.provider ?? candidate.source}</span>
          {episodeLabel(candidate) ? <span className="spec-badge spec-episode">{episodeLabel(candidate)}</span> : null}
          <span className={`spec-badge spec-res ${resolutionBadgeClass(candidate.release.resolution)}`}>{candidate.release.resolution}</span>
          <span className="spec-badge spec-codec">{candidate.release.videoCodec}</span>
          {candidate.release.dynamicRange ? <span className={`spec-badge spec-hdr ${dynamicRangeBadgeClass(candidate.release.dynamicRange)}`}>{candidate.release.dynamicRange}</span> : null}
        </span>
        <span className="result-facts">
          <span className="result-fact-item"><Volume2 aria-hidden="true" size={13} /><span>{candidate.release.audio ?? '音轨未知'}</span></span>
          <span>·</span>
          <span className="result-fact-item"><HardDrive aria-hidden="true" size={13} /><span>{formatCandidateSize(candidate.release.sizeBytes)}</span></span>
        </span>
      </button>
      <div className="result-action"><span className={`availability ${transferring ? 'transferring' : available ? 'available' : 'unavailable'}`}><span />{availabilityLabel}</span><div className="result-commands"><IconButton disabled={!subscribable} label="创建订阅" onClick={() => onSubscribe(candidate)}><BellPlus size={15} /></IconButton><button disabled={!available || transferring} onClick={() => onTransfer(candidate)} type="button">{submitting ? '提交中…' : transferring ? (downloadable ? '下载中' : '转存中') : downloadable ? '下载' : '转存'}{downloadable ? <Download size={15} /> : <FolderInput size={15} />}</button></div></div>
    </article>
  )
}

export function DiscoveryPosterBand({
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
