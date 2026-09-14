import { ExternalLink, Play } from 'lucide-react'
import type { EmbyItem } from '../../shared/api/mediaHub'
import { embyOpenLabel, libraryWatchLabel } from './libraryPlayback'

type LibraryWatchActionProps = {
  inPage: boolean
  name: string
  named?: boolean
  externalUrl?: string
  item: Pick<EmbyItem, 'played' | 'playbackPositionMs'>
  busy?: boolean
  onPlay: () => void
}

export function LibraryWatchAction({
  inPage,
  name,
  named = false,
  externalUrl,
  item,
  busy,
  onPlay,
}: LibraryWatchActionProps) {
  const label = libraryWatchLabel(inPage, item)
  const ariaLabel = named ? `${label} ${name}` : undefined
  if (inPage) {
    return (
      <button aria-label={ariaLabel} className="primary-action" disabled={busy} onClick={onPlay} type="button">
        <Play size={16} />
        {label}
      </button>
    )
  }
  if (!externalUrl) return null
  return (
    <a aria-label={ariaLabel} className="primary-action" href={externalUrl} rel="noreferrer" target="_blank">
      <ExternalLink size={16} />
      {embyOpenLabel}
    </a>
  )
}
