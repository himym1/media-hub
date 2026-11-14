import { BellPlus, Download, Film, FolderInput, HardDrive, Tv, Volume2, X } from 'lucide-react'
import type { Candidate, DiscoveryItem } from '../../shared/api/mediaHub'
import { IconButton } from '../../shared/ui/IconButton'
import {
  dynamicRangeBadgeClass,
  formatCandidateSize,
  isDownloadable,
  pipelineLabel,
  resolutionBadgeClass,
} from './discoverySearch'

type DiscoveryDetailPanelProps = {
  selected: Candidate
  detailPoster?: string
  detailRating?: number
  detailOverview?: string
  recommendations: DiscoveryItem[]
  isTransferPending: boolean
  onClose: () => void
  onOpenDiscoveryItem: (item: DiscoveryItem) => void
  onSubscribe: (candidate: Candidate) => void
  onTransfer: (candidate: Candidate) => void
}

export function DiscoveryDetailPanel({
  selected,
  detailPoster,
  detailRating,
  detailOverview,
  recommendations,
  isTransferPending,
  onClose,
  onOpenDiscoveryItem,
  onSubscribe,
  onTransfer,
}: DiscoveryDetailPanelProps) {
  const downloadable = isDownloadable(selected)
  return (
    <aside className="detail-panel" aria-label="资源详情">
      <div className="detail-header">
        <div><h2>{selected.title}</h2></div>
        <IconButton label="关闭详情" onClick={onClose} subtle><X size={17} /></IconButton>
      </div>
      <div className="detail-poster">
        {detailPoster ? (
          <>
            <div
              aria-hidden="true"
              className="detail-poster-backdrop"
              style={{ backgroundImage: `url(${detailPoster})` }}
            />
            <div aria-hidden="true" className="detail-poster-vignette" />
            <div className="detail-poster-foreground">
              <img alt={`${selected.title} 海报`} height="180" src={detailPoster} width="120" />
            </div>
          </>
        ) : (
          <div className="poster-placeholder"><Film size={34} /></div>
        )}
        <div className="poster-overlay">
          <span className={`spec-badge ${downloadable ? 'spec-pipeline-pt' : 'spec-pipeline-115'}`}>{pipelineLabel(selected)}</span>
          <span className="spec-badge spec-source">{selected.provider ?? selected.source}</span>
          <strong className={`spec-badge spec-res ${resolutionBadgeClass(selected.release.resolution)}`}>
            {selected.release.resolution}
          </strong>
        </div>
      </div>
      {detailRating || detailOverview ? (
        <div className="detail-identity">
          {detailRating ? <p className="detail-rating">TMDB {Number(detailRating).toFixed(1)}</p> : null}
          {detailOverview ? <p className="detail-overview">{detailOverview}</p> : null}
        </div>
      ) : null}
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
          <strong>{formatCandidateSize(selected.release.sizeBytes)}</strong>
        </div>
        <div className="detail-fact-card">
          <span>{downloadable ? <Download aria-hidden="true" size={13} /> : <FolderInput aria-hidden="true" size={13} />}{downloadable ? '下载目标' : '转存目标'}</span>
          <strong>{downloadable ? 'MoviePilot / 本地' : selected.mediaType === 'movie' ? '115 / 电影' : '115 / 电视剧'}</strong>
        </div>
      </div>
      {recommendations.length ? (
        <div className="recommendation-links">
          <span>相似内容</span>
          {recommendations.slice(0, 4).map((item) => (
            <button key={`${item.mediaType}-${item.tmdbId}`} onClick={() => onOpenDiscoveryItem(item)} type="button">
              {item.title}
            </button>
          ))}
        </div>
      ) : null}
      <div className="detail-actions">
        <button
          className="secondary-command"
          disabled={!selected.tmdbId || selected.transferState === 'identity_required'}
          onClick={() => onSubscribe(selected)}
          type="button"
        >
          <BellPlus size={16} />订阅
        </button>
        <button
          className="primary-action"
          disabled={!selected.transferToken || isTransferPending}
          onClick={() => onTransfer(selected)}
          type="button"
        >
          {downloadable ? <Download size={17} /> : <FolderInput size={17} />}
          {isTransferPending ? '正在创建任务…' : selected.transferToken ? (downloadable ? '加入下载队列' : '加入转存队列') : selected.transferState === 'identity_required' ? '身份待确认' : '工作流不可用'}
        </button>
      </div>
    </aside>
  )
}
