import { Maximize2, Pause, Play, X } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { ApiError, createEmbyPlaybackDescriptor, reportPlaybackSessionEvent } from '../../shared/api/mediaHub'
import { IconButton } from '../../shared/ui/IconButton'
import { isHlsStream } from './isHlsStream'
import { formatPlaybackClock } from './libraryPlayback'
import { attachPlaybackSession } from './libraryPlaybackSession'
import './LibraryPlayer.css'

type LibraryPlayerProps = {
  itemId: string
  title: string
  externalUrl: string
  onClose: () => void
}

export function LibraryPlayer({ itemId, title, externalUrl, onClose }: LibraryPlayerProps) {
  const videoRef = useRef<HTMLVideoElement>(null)
  const [error, setError] = useState<string | null>(null)
  const [playing, setPlaying] = useState(false)
  const [currentSeconds, setCurrentSeconds] = useState(0)
  const [durationSeconds, setDurationSeconds] = useState(0)

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault()
        onClose()
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  useEffect(() => {
    const video = videoRef.current
    if (!video) return
    let cancelled = false
    let hls: { destroy: () => void } | undefined
    let detachSession: () => void = () => {}
    const fail = () => {
      if (!cancelled) setError('当前浏览器无法直接播放')
    }
    video.addEventListener('error', fail)
    void (async () => {
      try {
        const descriptor = await createEmbyPlaybackDescriptor(itemId, navigator.userAgent)
        if (cancelled) return
        if (isHlsStream(descriptor.streamUrl)) {
          const { default: Hls } = await import('hls.js')
          if (Hls.isSupported()) {
            const player = new Hls()
            hls = player
            player.loadSource(descriptor.streamUrl)
            player.attachMedia(video)
          } else {
            video.src = descriptor.streamUrl
          }
        } else {
          video.src = descriptor.streamUrl
        }
        if ((descriptor.startPositionMs ?? 0) > 0) {
          const seek = () => {
            video.currentTime = (descriptor.startPositionMs ?? 0) / 1000
            video.removeEventListener('loadedmetadata', seek)
          }
          video.addEventListener('loadedmetadata', seek)
        }
        detachSession = attachPlaybackSession(descriptor.sessionId, video, reportPlaybackSessionEvent)
        await video.play().catch(() => undefined)
      } catch (cause) {
        if (!cancelled) {
          setError(cause instanceof ApiError ? cause.message : '当前浏览器无法直接播放')
        }
      }
    })()
    return () => {
      cancelled = true
      video.removeEventListener('error', fail)
      detachSession()
      hls?.destroy()
      video.removeAttribute('src')
      video.load()
    }
  }, [itemId])

  return (
    <div aria-labelledby="library-player-title" aria-modal="true" className="library-player" role="dialog">
      <div className="library-player-toolbar">
        <h2 id="library-player-title">{title}</h2>
        <IconButton label="关闭播放器" onClick={onClose}><X size={17} /></IconButton>
      </div>
      <video
        ref={videoRef}
        className="library-player-video"
        controls={false}
        onDurationChange={(event) => setDurationSeconds(event.currentTarget.duration || 0)}
        onPause={() => setPlaying(false)}
        onPlay={() => setPlaying(true)}
        onTimeUpdate={(event) => setCurrentSeconds(event.currentTarget.currentTime || 0)}
        playsInline
      />
      {error ? (
        <div className="library-player-error" role="alert">
          <strong>{error}</strong>
          <a className="primary-action" href={externalUrl} rel="noreferrer" target="_blank">在 Emby 打开</a>
        </div>
      ) : null}
      <div className="library-player-controls">
        <button
          aria-label={playing ? '暂停' : '播放'}
          className="primary-action"
          onClick={() => {
            const video = videoRef.current
            if (!video) return
            if (video.paused) void video.play().catch(() => undefined)
            else video.pause()
          }}
          type="button"
        >
          {playing ? <Pause size={16} /> : <Play size={16} />}
          {playing ? '暂停' : '播放'}
        </button>
        <label className="library-player-seek">
          <span>{formatPlaybackClock(currentSeconds)}</span>
          <input
            aria-label="播放进度"
            max={durationSeconds || 0}
            min={0}
            onChange={(event) => {
              const video = videoRef.current
              if (video) video.currentTime = Number(event.target.value)
            }}
            step={1}
            type="range"
            value={Math.min(currentSeconds, durationSeconds || 0)}
          />
          <span>{formatPlaybackClock(durationSeconds)}</span>
        </label>
        <button
          className="secondary-command"
          onClick={() => void videoRef.current?.requestFullscreen()}
          type="button"
        >
          <Maximize2 size={16} />
          全屏
        </button>
      </div>
    </div>
  )
}
