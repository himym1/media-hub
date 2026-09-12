import { AudioLines, Captions, Maximize2, Pause, PictureInPicture2, Play, RotateCcw, RotateCw, SkipForward, Volume2, VolumeX, X, ZoomIn, ZoomOut } from 'lucide-react'
import { useCallback, useEffect, useRef, useState, type CSSProperties } from 'react'
import { ApiError, createEmbyPlaybackDescriptor, fetchLocalSubtitle, reportPlaybackSessionEvent } from '../../shared/api/mediaHub'
import {
  boundsFromElement,
  canPlayNatively,
  controlNatively,
  layoutNatively,
  nativeStatus,
  nativeSubtitleFromBytes,
  playNatively,
  stopNatively,
  type NativeSubtitle,
} from '../../shared/desktop/nativePlayback'
import { IconButton } from '../../shared/ui/IconButton'
import { isHlsStream } from './isHlsStream'
import { formatPlaybackClock } from './libraryPlayback'
import { attachPlaybackSession } from './libraryPlaybackSession'
import { attachSubtitleTrack, setSubtitleMode, subtitleAttachResult } from './librarySubtitle'
import {
  clampPictureZoom,
  formatPictureZoom,
  isPlayerAspectId,
  nextPlayerAspect,
  playbackRates,
  playbackSkipSeconds,
  playerAspectClassName,
  playerAspectModes,
  stepPictureZoom,
  type PlayerAspectId,
} from './playerChrome'
import { looksLikeSilentDirectPlay } from './silentAudio'
import './LibraryPlayer.css'

const chromeIdleMs = 2800

type LibraryPlayerProps = {
  itemId: string
  title: string
  externalUrl: string
  nextEpisodeLabel?: string
  onClose: () => void
  onNextEpisode?: () => void
}

type NativeRequest = {
  url: string
  title: string
  startPositionMs?: number
  userAgent?: string
  subtitle?: NativeSubtitle | null
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
  const holeRef = useRef<HTMLDivElement>(null)
  const stageRef = useRef<HTMLDivElement>(null)
  const idleTimer = useRef<number | null>(null)
  const nativeRequest = useRef<NativeRequest | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [playing, setPlaying] = useState(false)
  const [muted, setMuted] = useState(false)
  const [volume, setVolume] = useState(1)
  const [rate, setRate] = useState(1)
  const [currentSeconds, setCurrentSeconds] = useState(0)
  const [durationSeconds, setDurationSeconds] = useState(0)
  const [captions, setCaptions] = useState<'off' | 'on' | 'missing' | 'ass'>('missing')
  const [pipAvailable, setPipAvailable] = useState(false)
  const [chromeVisible, setChromeVisible] = useState(true)
  const [chromePinned, setChromePinned] = useState(false)
  const [allowAutoHide, setAllowAutoHide] = useState(false)
  const [silentAudio, setSilentAudio] = useState(false)
  const [nativeActive, setNativeActive] = useState(false)
  const [aspect, setAspect] = useState<PlayerAspectId>('fit')
  const [pictureZoom, setPictureZoom] = useState(1)

  const clearIdleTimer = () => {
    if (idleTimer.current != null) {
      window.clearTimeout(idleTimer.current)
      idleTimer.current = null
    }
  }

  const revealChrome = useCallback((sticky = false, armHide = false) => {
    setChromeVisible(true)
    if (sticky) setChromePinned(true)
    if (armHide) setAllowAutoHide(true)
    clearIdleTimer()
    if (nativeActive || !playing || sticky || (!armHide && !allowAutoHide)) return
    idleTimer.current = window.setTimeout(() => {
      setChromeVisible(false)
    }, chromeIdleMs)
  }, [allowAutoHide, nativeActive, playing])

  useEffect(() => {
    setPipAvailable(Boolean(document.pictureInPictureEnabled))
  }, [])

  useEffect(() => () => clearIdleTimer(), [])

  useEffect(() => {
    if (nativeActive || !playing || chromePinned || error) {
      clearIdleTimer()
      setChromeVisible(true)
      return
    }
    revealChrome()
  }, [playing, chromePinned, error, nativeActive, revealChrome])

  const seekBy = useCallback((delta: number) => {
    if (nativeActive) {
      void controlNatively('seek', Math.max(0, currentSeconds + delta)).catch(() => undefined)
      return
    }
    const video = videoRef.current
    if (!video) return
    const duration = video.duration || video.currentTime + Math.abs(delta)
    video.currentTime = Math.min(duration, Math.max(0, video.currentTime + delta))
  }, [currentSeconds, nativeActive])

  const applyAspect = useCallback((next: PlayerAspectId) => {
    setAspect(next)
    if (nativeActive) void controlNatively('aspect', undefined, next).catch(() => undefined)
  }, [nativeActive])

  const applyZoom = useCallback((next: number) => {
    const zoom = clampPictureZoom(next)
    setPictureZoom(zoom)
    if (nativeActive) void controlNatively('zoom', zoom).catch(() => undefined)
  }, [nativeActive])

  const togglePlayback = useCallback(() => {
    if (nativeActive) {
      void controlNatively('cycle-pause').catch(() => undefined)
      return
    }
    const video = videoRef.current
    if (!video) return
    if (video.paused) void video.play().catch(() => undefined)
    else video.pause()
  }, [nativeActive])

  const toggleFullscreen = useCallback(() => {
    const root = holeRef.current?.closest('.library-player') ?? videoRef.current
    if (!root) return
    if (document.fullscreenElement) void document.exitFullscreen()
    else void root.requestFullscreen()
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
      revealChrome()
      if (event.key === ' ' || event.key === 'k') {
        event.preventDefault()
        togglePlayback()
      } else if (event.key === 'ArrowLeft' || event.key === 'j') {
        event.preventDefault()
        seekBy(-playbackSkipSeconds)
      } else if (event.key === 'ArrowRight' || event.key === 'l') {
        event.preventDefault()
        seekBy(playbackSkipSeconds)
      } else if (event.key === 'f') {
        event.preventDefault()
        toggleFullscreen()
      } else if (event.key === 'm') {
        event.preventDefault()
        if (nativeActive) void controlNatively('mute').catch(() => undefined)
        else if (video) {
          video.muted = !video.muted
          setMuted(video.muted)
        }
      } else if (event.key === 'c') {
        event.preventDefault()
        if (captions === 'on' || captions === 'off') {
          const next = captions === 'on' ? 'off' : 'on'
          if (nativeActive) void controlNatively('subtitles').catch(() => undefined)
          else if (video) setSubtitleMode(video, next === 'on')
          setCaptions(next)
        }
      } else if (event.key === 'z') {
        event.preventDefault()
        applyAspect(nextPlayerAspect(aspect))
      } else if (event.key === '-' || event.key === '_') {
        event.preventDefault()
        applyZoom(stepPictureZoom(pictureZoom, -1))
      } else if (event.key === '=' || event.key === '+') {
        event.preventDefault()
        applyZoom(stepPictureZoom(pictureZoom, 1))
      } else if (event.key === '0') {
        event.preventDefault()
        applyZoom(1)
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [applyAspect, applyZoom, aspect, captions, nativeActive, onClose, pictureZoom, revealChrome, seekBy, toggleFullscreen, togglePlayback])

  useEffect(() => {
    setError(null)
    setSilentAudio(false)
    setAllowAutoHide(false)
    setChromeVisible(true)
    setChromePinned(false)
    setNativeActive(false)
    setAspect('fit')
    setPictureZoom(1)
    nativeRequest.current = null
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
        if (canPlayNatively()) {
          const nativeSubtitle = subtitle
            ? nativeSubtitleFromBytes(subtitle.bytes, subtitle.fileName)
            : null
          nativeRequest.current = {
            url: descriptor.streamUrl,
            title,
            startPositionMs: descriptor.startPositionMs,
            userAgent: descriptor.userAgent,
            subtitle: nativeSubtitle,
          }
          setCaptions(nativeSubtitle ? 'on' : 'missing')
          setNativeActive(true)
          return
        }
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
  }, [itemId, title])

  useEffect(() => {
    if (!nativeActive || !nativeRequest.current) return
    const hole = holeRef.current
    if (!hole) return
    let cancelled = false
    const request = nativeRequest.current
    void (async () => {
      try {
        await playNatively(request.url, {
          title: request.title,
          startPositionMs: request.startPositionMs,
          userAgent: request.userAgent,
          bounds: boundsFromElement(hole),
          subtitle: request.subtitle,
        })
        if (!cancelled) setPlaying(true)
      } catch (cause) {
        if (!cancelled) {
          setError(cause instanceof Error ? cause.message : '系统播放器未能打开这路流。')
        }
      }
    })()
    const relayout = () => {
      void layoutNatively(boundsFromElement(hole)).catch(() => undefined)
    }
    const observer = new ResizeObserver(relayout)
    observer.observe(hole)
    window.addEventListener('resize', relayout)
    const poll = window.setInterval(() => {
      void nativeStatus().then((status) => {
        if (!status || cancelled) return
        setPlaying(!status.paused)
        setCurrentSeconds(status.time)
        setDurationSeconds(status.duration)
        setVolume(status.volume)
        setRate(status.speed)
        if (typeof status.zoom === 'number' && status.zoom > 0) {
          setPictureZoom(clampPictureZoom(status.zoom))
        }
      })
    }, 500)
    return () => {
      cancelled = true
      observer.disconnect()
      window.removeEventListener('resize', relayout)
      window.clearInterval(poll)
      void stopNatively().catch(() => undefined)
    }
  }, [itemId, nativeActive])

  useEffect(() => {
    if (error || silentAudio || nativeActive) return
    const video = videoRef.current
    if (!video) return
    const timer = window.setInterval(() => {
      if (looksLikeSilentDirectPlay(video)) {
        setSilentAudio(true)
        setChromeVisible(true)
        window.clearInterval(timer)
      }
    }, 1000)
    return () => window.clearInterval(timer)
  }, [itemId, error, silentAudio, playing, nativeActive])

  useEffect(() => {
    const stage = stageRef.current
    if (!stage || nativeActive) return
    const onWheel = (event: WheelEvent) => {
      event.preventDefault()
      revealChrome(false, true)
      setPictureZoom((current) => clampPictureZoom(current + (event.deltaY > 0 ? -0.1 : 0.1)))
    }
    stage.addEventListener('wheel', onWheel, { passive: false })
    return () => stage.removeEventListener('wheel', onWheel)
  }, [nativeActive, revealChrome])

  const toggleCaptions = () => {
    if (captions !== 'on' && captions !== 'off') return
    const next = captions === 'on' ? 'off' : 'on'
    if (nativeActive) {
      void controlNatively('subtitles').catch(() => undefined)
      setCaptions(next)
      return
    }
    const video = videoRef.current
    if (!video) return
    setSubtitleMode(video, next === 'on')
    setCaptions(next)
  }

  const seekProgress = durationSeconds > 0 ? Math.min(1, currentSeconds / durationSeconds) : 0
  const shellClass = [
    'library-player',
    nativeActive ? 'is-native' : '',
    nativeActive || chromeVisible || !playing || Boolean(error) ? 'chrome-visible' : 'chrome-hidden',
    playing ? 'is-playing' : 'is-paused',
  ].filter(Boolean).join(' ')

  return (
    <div
      aria-labelledby="library-player-title"
      aria-modal="true"
      className={shellClass}
      onMouseMove={() => revealChrome(false, true)}
      onPointerDown={() => revealChrome(false, true)}
      role="dialog"
    >
      <div className="library-player-stage" ref={stageRef}>
        <header
          className="library-player-toolbar"
          onBlurCapture={(event) => {
            if (!event.currentTarget.contains(event.relatedTarget as Node | null)) {
              setChromePinned(false)
              revealChrome()
            }
          }}
          onFocusCapture={() => revealChrome(true)}
          onMouseEnter={() => revealChrome(true)}
          onMouseLeave={() => {
            setChromePinned(false)
            revealChrome()
          }}
        >
          <h2 id="library-player-title">{title}</h2>
          <div className="library-player-toolbar-actions">
            <label className="library-player-aspect">
              <span>画面</span>
              <select
                aria-label="画面比例"
                onChange={(event) => {
                  if (!isPlayerAspectId(event.target.value)) return
                  applyAspect(event.target.value)
                  revealChrome(true)
                }}
                title={playerAspectModes.find((mode) => mode.id === aspect)?.description}
                value={aspect}
              >
                {playerAspectModes.map((mode) => (
                  <option key={mode.id} value={mode.id}>{mode.label}</option>
                ))}
              </select>
            </label>
            <button
              aria-label="缩小画面"
              className="library-player-icon-action"
              onClick={() => applyZoom(stepPictureZoom(pictureZoom, -1))}
              type="button"
            >
              <ZoomOut size={18} />
            </button>
            <span aria-label={`画面缩放 ${formatPictureZoom(pictureZoom)}`} className="library-player-zoom-value">
              {formatPictureZoom(pictureZoom)}
            </span>
            <button
              aria-label="放大画面"
              className="library-player-icon-action"
              onClick={() => applyZoom(stepPictureZoom(pictureZoom, 1))}
              type="button"
            >
              <ZoomIn size={18} />
            </button>
            <IconButton label="关闭播放器" onClick={onClose}><X size={17} /></IconButton>
          </div>
        </header>

        {nativeActive ? <div aria-hidden="true" className="library-player-native-hole" ref={holeRef} /> : null}

        <div
          className={`library-player-picture ${playerAspectClassName(aspect)}`}
          style={{ '--picture-zoom': String(pictureZoom) } as CSSProperties}
        >
          <video
            ref={videoRef}
            className="library-player-video"
            controls={false}
            onClick={() => {
              revealChrome()
              togglePlayback()
            }}
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
        </div>

        {!playing && !error && !nativeActive ? (
          <button aria-label="继续播放" className="library-player-center-play" onClick={togglePlayback} type="button">
            <Play size={28} />
          </button>
        ) : null}

        {error ? (
          <div className="library-player-error" role="alert">
            <strong>{error}</strong>
            <a className="primary-action" href={externalUrl} rel="noreferrer" target="_blank">在 Emby 打开</a>
          </div>
        ) : null}

        {captions === 'ass' && !nativeActive ? (
          <p className="library-player-note" role="status">
            当前是 ASS 字幕，浏览器无法渲染。
            <a href={externalUrl} rel="noreferrer" target="_blank">在 Emby 打开</a>
          </p>
        ) : null}

        {silentAudio && !error && !nativeActive ? (
          <p className="library-player-note" role="status">
            检测到有画面但浏览器未解码出声音。自己的库是直出原片，DTS / TrueHD / 部分 EAC3 音轨在 Chrome 等浏览器里常会静音。
            <a href={externalUrl} rel="noreferrer" target="_blank">在 Emby 打开</a>
          </p>
        ) : null}

        <div
          className="library-player-chrome"
          onFocusCapture={() => revealChrome(true)}
          onBlurCapture={(event) => {
            if (!event.currentTarget.contains(event.relatedTarget as Node | null)) {
              setChromePinned(false)
              revealChrome()
            }
          }}
          onMouseEnter={() => revealChrome(true)}
          onMouseLeave={() => {
            setChromePinned(false)
            revealChrome()
          }}
        >
          <label className="library-player-seek" style={{ '--seek-progress': `${seekProgress * 100}%` } as CSSProperties}>
            <span>{formatPlaybackClock(currentSeconds)}</span>
            <input
              aria-label="播放进度"
              max={durationSeconds || 0}
              min={0}
              onChange={(event) => {
                const next = Number(event.target.value)
                if (nativeActive) void controlNatively('seek', next).catch(() => undefined)
                else if (videoRef.current) videoRef.current.currentTime = next
                revealChrome(true)
              }}
              step={1}
              type="range"
              value={Math.min(currentSeconds, durationSeconds || 0)}
            />
            <span>{formatPlaybackClock(durationSeconds)}</span>
          </label>

          <div className="library-player-controls">
            <button
              aria-label="后退 10 秒"
              className="library-player-icon-action"
              onClick={() => seekBy(-playbackSkipSeconds)}
              type="button"
            >
              <RotateCcw size={18} />
              <span aria-hidden="true">-10s</span>
            </button>

            <button
              aria-label={playing ? '暂停' : '播放'}
              className="library-player-icon-action"
              onClick={togglePlayback}
              type="button"
            >
              {playing ? <Pause size={18} /> : <Play size={18} />}
              <span aria-hidden="true">{playing ? '暂停' : '播放'}</span>
            </button>

            <button
              aria-label="前进 10 秒"
              className="library-player-icon-action"
              onClick={() => seekBy(playbackSkipSeconds)}
              type="button"
            >
              <RotateCw size={18} />
              <span aria-hidden="true">+10s</span>
            </button>

            <button
              aria-label={muted || volume === 0 ? '取消静音' : '静音'}
              className="library-player-icon-action"
              onClick={() => {
                if (nativeActive) {
                  void controlNatively('mute').catch(() => undefined)
                  setMuted((current) => !current)
                  return
                }
                const video = videoRef.current
                if (!video) return
                video.muted = !video.muted
                setMuted(video.muted)
              }}
              type="button"
            >
              {muted || volume === 0 ? <VolumeX size={18} /> : <Volume2 size={18} />}
            </button>
            <label className="library-player-volume">
              <span>音量</span>
              <input
                aria-label="音量"
                max={1}
                min={0}
                onChange={(event) => {
                  const next = Number(event.target.value)
                  if (nativeActive) {
                    void controlNatively('volume', next).catch(() => undefined)
                    setVolume(next)
                    setMuted(next === 0)
                    return
                  }
                  const video = videoRef.current
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
                  const next = Number(event.target.value)
                  if (nativeActive) void controlNatively('speed', next).catch(() => undefined)
                  else if (videoRef.current) videoRef.current.playbackRate = next
                  setRate(next)
                }}
                value={String(rate)}
              >
                {playbackRates.map((option) => (
                  <option key={option} value={option}>{option === 1 ? '1×' : `${option}×`}</option>
                ))}
              </select>
            </label>

            {(captions === 'on' || captions === 'off') ? (
              <button
                aria-label={captions === 'on' ? '字幕开' : '字幕关'}
                aria-pressed={captions === 'on'}
                className={captions === 'on' ? 'library-player-icon-action active' : 'library-player-icon-action'}
                onClick={toggleCaptions}
                type="button"
              >
                <Captions size={18} />
                <span aria-hidden="true">{captions === 'on' ? '字幕开' : '字幕关'}</span>
              </button>
            ) : !nativeActive ? (
              <button
                aria-label="无字幕"
                aria-pressed={false}
                className="library-player-icon-action"
                disabled
                type="button"
              >
                <Captions size={18} />
                <span aria-hidden="true">无字幕</span>
              </button>
            ) : null}

            {nativeActive ? (
              <button
                aria-label="切换音轨"
                className="library-player-icon-action"
                onClick={() => void controlNatively('cycle-audio').catch(() => undefined)}
                type="button"
              >
                <AudioLines size={18} />
                <span aria-hidden="true">音轨</span>
              </button>
            ) : null}

            {pipAvailable && !nativeActive ? (
              <button
                aria-label="画中画"
                className="library-player-icon-action"
                onClick={() => {
                  const video = videoRef.current
                  if (!video) return
                  if (document.pictureInPictureElement) void document.exitPictureInPicture()
                  else void video.requestPictureInPicture()
                }}
                type="button"
              >
                <PictureInPicture2 size={18} />
                <span aria-hidden="true">画中画</span>
              </button>
            ) : null}

            <button
              aria-label="全屏"
              className="library-player-icon-action"
              onClick={toggleFullscreen}
              type="button"
            >
              <Maximize2 size={18} />
              <span aria-hidden="true">全屏</span>
            </button>

            {onNextEpisode && nextEpisodeLabel ? (
              <button className="library-player-next" onClick={onNextEpisode} type="button">
                <SkipForward size={16} />
                下一集 {nextEpisodeLabel}
              </button>
            ) : null}
          </div>
        </div>
      </div>
    </div>
  )
}
