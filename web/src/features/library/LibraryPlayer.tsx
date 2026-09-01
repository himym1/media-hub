import { Captions, Maximize2, Pause, PictureInPicture2, Play, Volume2, VolumeX, X } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { ApiError, createEmbyPlaybackDescriptor, fetchLocalSubtitle, reportPlaybackSessionEvent } from '../../shared/api/mediaHub'
import { IconButton } from '../../shared/ui/IconButton'
import { isHlsStream } from './isHlsStream'
import { formatPlaybackClock } from './libraryPlayback'
import { attachPlaybackSession } from './libraryPlaybackSession'
import { attachSubtitleTrack, setSubtitleMode, subtitleAttachResult } from './librarySubtitle'
import './LibraryPlayer.css'

const playbackRates = [0.75, 1, 1.25, 1.5, 2]

type LibraryPlayerProps = {
  itemId: string
  title: string
  externalUrl: string
  nextEpisodeLabel?: string
  onClose: () => void
  onNextEpisode?: () => void
}

export function LibraryPlayer({
  itemId,
  title,
  externalUrl,
  nextEpisodeLabel,
  onClose,
  onNextEpisode,
}: LibraryPlayerProps) {
  const videoRef = useRef<HTMLVideoElement>(null)
  const [error, setError] = useState<string | null>(null)
  const [playing, setPlaying] = useState(false)
  const [muted, setMuted] = useState(false)
  const [volume, setVolume] = useState(1)
  const [rate, setRate] = useState(1)
  const [currentSeconds, setCurrentSeconds] = useState(0)
  const [durationSeconds, setDurationSeconds] = useState(0)
  const [captions, setCaptions] = useState<'off' | 'on' | 'missing' | 'ass'>('missing')
  const [pipAvailable, setPipAvailable] = useState(false)

  useEffect(() => {
    setPipAvailable(Boolean(document.pictureInPictureEnabled))
  }, [])

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      const tag = (event.target as HTMLElement | null)?.tagName
      if (tag === 'INPUT' || tag === 'SELECT' || tag === 'TEXTAREA') return
      const video = videoRef.current
      if (event.key === 'Escape') {
        event.preventDefault()
        onClose()
        return
      }
      if (!video) return
      if (event.key === ' ' || event.key === 'k') {
        event.preventDefault()
        if (video.paused) void video.play().catch(() => undefined)
        else video.pause()
      } else if (event.key === 'ArrowLeft') {
        event.preventDefault()
        video.currentTime = Math.max(0, video.currentTime - 5)
      } else if (event.key === 'ArrowRight') {
        event.preventDefault()
        video.currentTime = Math.min(video.duration || video.currentTime + 5, video.currentTime + 5)
      } else if (event.key === 'f') {
        event.preventDefault()
        if (document.fullscreenElement) void document.exitFullscreen()
        else void video.requestFullscreen()
      } else if (event.key === 'm') {
        event.preventDefault()
        video.muted = !video.muted
        setMuted(video.muted)
      } else if (event.key === 'c') {
        event.preventDefault()
        if (captions === 'on' || captions === 'off') {
          const next = captions === 'on' ? 'off' : 'on'
          setSubtitleMode(video, next === 'on')
          setCaptions(next)
        }
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [captions, onClose])

  useEffect(() => {
    const video = videoRef.current
    if (!video) return
    let cancelled = false
    let hls: { destroy: () => void } | undefined
    let detachSession: () => void = () => {}
    let detachSubtitle: () => void = () => {}
    let subtitleUrl: string | undefined
    const fail = () => {
      if (!cancelled) setError('当前浏览器无法直接播放')
    }
    video.addEventListener('error', fail)
    void (async () => {
      try {
        const [descriptor, subtitle] = await Promise.all([
          createEmbyPlaybackDescriptor(itemId, navigator.userAgent),
          fetchLocalSubtitle(itemId).catch(() => null),
        ])
        if (cancelled) return
        const attached = subtitleAttachResult(subtitle)
        if (attached.kind === 'track') {
          subtitleUrl = attached.url
          detachSubtitle = attachSubtitleTrack(video, attached.url)
          setCaptions('on')
        } else if (attached.kind === 'ass') {
          setCaptions('ass')
        } else {
          setCaptions('missing')
        }
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
      detachSubtitle()
      if (subtitleUrl) URL.revokeObjectURL(subtitleUrl)
      hls?.destroy()
      video.removeAttribute('src')
      video.load()
    }
  }, [itemId])

  const toggleCaptions = () => {
    const video = videoRef.current
    if (!video || (captions !== 'on' && captions !== 'off')) return
    const next = captions === 'on' ? 'off' : 'on'
    setSubtitleMode(video, next === 'on')
    setCaptions(next)
  }

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
        onRateChange={(event) => setRate(event.currentTarget.playbackRate || 1)}
        onTimeUpdate={(event) => setCurrentSeconds(event.currentTarget.currentTime || 0)}
        onVolumeChange={(event) => {
          setMuted(event.currentTarget.muted)
          setVolume(event.currentTarget.volume)
        }}
        playsInline
      />
      {error ? (
        <div className="library-player-error" role="alert">
          <strong>{error}</strong>
          <a className="primary-action" href={externalUrl} rel="noreferrer" target="_blank">在 Emby 打开</a>
        </div>
      ) : null}
      {captions === 'ass' ? (
        <p className="library-player-note" role="status">
          当前是 ASS 字幕，浏览器无法渲染。
          <a href={externalUrl} rel="noreferrer" target="_blank">在 Emby 打开</a>
        </p>
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
      <div className="library-player-tools">
        <button
          aria-label={muted || volume === 0 ? '取消静音' : '静音'}
          className="secondary-command"
          onClick={() => {
            const video = videoRef.current
            if (!video) return
            video.muted = !video.muted
            setMuted(video.muted)
          }}
          type="button"
        >
          {muted || volume === 0 ? <VolumeX size={16} /> : <Volume2 size={16} />}
        </button>
        <label className="library-player-volume">
          <span>音量</span>
          <input
            aria-label="音量"
            max={1}
            min={0}
            onChange={(event) => {
              const video = videoRef.current
              const next = Number(event.target.value)
              if (!video) return
              video.volume = next
              video.muted = next === 0
              setVolume(next)
              setMuted(next === 0)
            }}
            step={0.05}
            type="range"
            value={muted ? 0 : volume}
          />
        </label>
        <label className="library-player-rate">
          <span>倍速</span>
          <select
            aria-label="播放速度"
            onChange={(event) => {
              const video = videoRef.current
              const next = Number(event.target.value)
              if (video) video.playbackRate = next
              setRate(next)
            }}
            value={String(rate)}
          >
            {playbackRates.map((option) => (
              <option key={option} value={option}>{option === 1 ? '1×' : `${option}×`}</option>
            ))}
          </select>
        </label>
        <button
          aria-pressed={captions === 'on'}
          className="secondary-command"
          disabled={captions === 'missing' || captions === 'ass'}
          onClick={toggleCaptions}
          type="button"
        >
          <Captions size={16} />
          {captions === 'on' ? '字幕开' : captions === 'off' ? '字幕关' : '无字幕'}
        </button>
        {pipAvailable ? (
          <button
            className="secondary-command"
            onClick={() => {
              const video = videoRef.current
              if (!video) return
              if (document.pictureInPictureElement) void document.exitPictureInPicture()
              else void video.requestPictureInPicture()
            }}
            type="button"
          >
            <PictureInPicture2 size={16} />
            画中画
          </button>
        ) : null}
        {onNextEpisode && nextEpisodeLabel ? (
          <button className="secondary-command" onClick={onNextEpisode} type="button">
            下一集 {nextEpisodeLabel}
          </button>
        ) : null}
      </div>
    </div>
  )
}
