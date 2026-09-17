import { Captions, Maximize2, Pause, PictureInPicture2, Play, RotateCcw, RotateCw, SkipForward, Volume2, VolumeX, X, ZoomIn, ZoomOut } from 'lucide-react'
import { useCallback, useEffect, useRef, useState, type CSSProperties } from 'react'
import { ApiError, createEmbyPlaybackDescriptor, fetchLocalSubtitle, reportPlaybackSessionEvent } from '../../shared/api/mediaHub'
import {
  attachNativeSubtitle,
  canPlayNatively,
  controlNatively,
  nativeStatus,
  nativeSubtitleFromBytes,
  playNatively,
  stopNatively,
  type NativeSubtitle,
} from '../../shared/desktop/nativePlayback'
import { IconButton } from '../../shared/ui/IconButton'
import { isHlsStream } from './isHlsStream'
import { formatPlaybackClock } from './libraryPlayback'
import { attachNativePlaybackSession, attachPlaybackSession } from './libraryPlaybackSession'
import { attachSubtitleTrack, setSubtitleMode, subtitleAttachResult, subtitleStartHint } from './librarySubtitle'
import {
  cancelScheduledPlayerClick,
  clampPictureZoom,
  formatPictureZoom,
  isPlayerAspectId,
  nextPlayerAspect,
  playbackRates,
  playbackSkipSeconds,
  playerAspectClassName,
  playerAspectModes,
  schedulePlayerClick,
  shouldAutoHidePlayerChrome,
  stepPictureZoom,
  type PlayerAspectId,
} from './playerChrome'
import {
  documentFullscreenElement,
  exitDocumentFullscreen,
  shouldClosePlayerOnEscape,
  toggleDocumentFullscreen,
} from './playerFullscreen'
import { handoffEnded, handoffStatusText } from './playerHandoff'
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
  sessionId?: string
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
  const nativeShell = canPlayNatively()
  const videoRef = useRef<HTMLVideoElement>(null)
  const toolbarRef = useRef<HTMLElement>(null)
  const chromeBarRef = useRef<HTMLDivElement>(null)
  const stageRef = useRef<HTMLDivElement>(null)
  const chromeVisibleRef = useRef(true)
  const idleTimer = useRef<number | null>(null)
  const clickTimer = useRef<number | null>(null)
  const nativeRequest = useRef<NativeRequest | null>(null)
  const nativeClock = useRef({ seconds: 0, paused: false })
  const nativeStarted = useRef(false)
  const nativeSeenRunning = useRef(false)
  const onCloseRef = useRef(onClose)
  onCloseRef.current = onClose
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
  const [webFullscreen, setWebFullscreen] = useState(false)
  const [subtitleHint, setSubtitleHint] = useState<string | null>(null)
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
    if (!playing || sticky || (!armHide && !allowAutoHide)) return
    idleTimer.current = window.setTimeout(() => {
      setChromeVisible(false)
    }, chromeIdleMs)
  }, [allowAutoHide, playing])
  chromeVisibleRef.current = chromeVisible

  useEffect(() => {
    setPipAvailable(Boolean(document.pictureInPictureEnabled))
  }, [])

  useEffect(() => () => {
    clearIdleTimer()
    cancelScheduledPlayerClick(clickTimer)
  }, [])

  useEffect(() => {
    if (nativeShell) return
    if (!shouldAutoHidePlayerChrome(false, playing, chromePinned, Boolean(error))) {
      clearIdleTimer()
      setChromeVisible(true)
      return
    }
    revealChrome(false, true)
  }, [chromePinned, error, nativeShell, playing, revealChrome])

  const seekBy = useCallback((delta: number) => {
    const video = videoRef.current
    if (!video) return
    const duration = video.duration || video.currentTime + Math.abs(delta)
    video.currentTime = Math.min(duration, Math.max(0, video.currentTime + delta))
  }, [])

  const applyAspect = useCallback((next: PlayerAspectId) => {
    setAspect(next)
  }, [])

  const applyZoom = useCallback((next: number) => {
    setPictureZoom(clampPictureZoom(next))
  }, [])

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

  const toggleCaptions = useCallback(() => {
    const next = captions === 'off' ? 'on' : 'off'
    if (captions !== 'on' && captions !== 'off') return
    const video = videoRef.current
    if (!video) return
    setSubtitleMode(video, next === 'on')
    setCaptions(next)
  }, [captions])

  const togglePresentation = useCallback(() => {
    const root = stageRef.current?.closest('.library-player') ?? videoRef.current
    if (!root) return
    void toggleDocumentFullscreen(root).catch(() => undefined)
  }, [])

  const onSurfaceClick = useCallback(() => {
    revealChrome(false, true)
    schedulePlayerClick(clickTimer, togglePlayback)
  }, [revealChrome, togglePlayback])

  const onSurfaceDoubleClick = useCallback(() => {
    cancelScheduledPlayerClick(clickTimer)
    revealChrome(false, true)
    togglePresentation()
  }, [revealChrome, togglePresentation])

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      const tag = (event.target as HTMLElement | null)?.tagName
      if (tag === 'INPUT' || tag === 'SELECT' || tag === 'TEXTAREA') return
      const video = videoRef.current
      if (event.key === 'Escape') {
        const inDocumentFullscreen = documentFullscreenElement() !== null
        if (!shouldClosePlayerOnEscape(false, inDocumentFullscreen)) {
          event.preventDefault()
          void exitDocumentFullscreen().catch(() => undefined)
          return
        }
        event.preventDefault()
        onClose()
        return
      }
      // mpv 自己一个窗口，播放键都归它，Hub 只留 Esc 收面板。
      if (nativeShell) return
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
        togglePresentation()
      } else if (event.key === 'm') {
        event.preventDefault()
        if (video) {
          video.muted = !video.muted
          setMuted(video.muted)
        }
      } else if (event.key === 'c') {
        event.preventDefault()
        toggleCaptions()
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
  }, [applyAspect, applyZoom, aspect, nativeActive, nativeShell, onClose, pictureZoom, revealChrome, seekBy, toggleCaptions, togglePlayback, togglePresentation])

  useEffect(() => {
    const sync = () => setWebFullscreen(documentFullscreenElement() !== null)
    sync()
    document.addEventListener('fullscreenchange', sync)
    document.addEventListener('webkitfullscreenchange', sync)
    return () => {
      document.removeEventListener('fullscreenchange', sync)
      document.removeEventListener('webkitfullscreenchange', sync)
    }
  }, [])

  useEffect(() => {
    setError(null)
    setSilentAudio(false)
    setAllowAutoHide(false)
    setChromeVisible(true)
    setChromePinned(false)
    setNativeActive(false)
    setSubtitleHint(null)
    setAspect('fit')
    setPictureZoom(1)
    nativeRequest.current = null
    nativeStarted.current = false
    let cancelled = false
    if (nativeShell) {
      void (async () => {
        try {
          const descriptor = await createEmbyPlaybackDescriptor(itemId, navigator.userAgent)
          if (cancelled) return
          nativeRequest.current = {
            url: descriptor.streamUrl,
            title,
            startPositionMs: descriptor.startPositionMs,
            userAgent: descriptor.userAgent,
            sessionId: descriptor.sessionId,
          }
          setNativeActive(true)
          const subtitle = await fetchLocalSubtitle(itemId).catch(() => null)
          if (cancelled || !subtitle) return
          const nativeSubtitle = nativeSubtitleFromBytes(subtitle.bytes, subtitle.fileName)
          if (!nativeSubtitle) return
          setSubtitleHint(subtitleStartHint(subtitle.bytes))
          if (nativeRequest.current && !nativeStarted.current) {
            nativeRequest.current.subtitle = nativeSubtitle
            return
          }
          void attachNativeSubtitle(nativeSubtitle).catch(() => undefined)
        } catch (cause) {
          if (!cancelled) {
            setError(cause instanceof ApiError ? cause.message : '系统播放器未能打开这路流。')
          }
        }
      })()
      return () => {
        cancelled = true
      }
    }
    const video = videoRef.current
    if (!video) return
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
        const descriptor = await createEmbyPlaybackDescriptor(itemId, navigator.userAgent)
        if (cancelled) return
        const subtitlePromise = fetchLocalSubtitle(itemId).catch(() => null)
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
        const subtitle = await subtitlePromise
        if (cancelled || !subtitle) return
        const attached = subtitleAttachResult(subtitle)
        setSubtitleHint(attached.kind === 'track' ? subtitleStartHint(subtitle.bytes) : null)
        if (attached.kind === 'track') {
          subtitleUrl = attached.url
          detachSubtitle = attachSubtitleTrack(video, attached.url)
          setCaptions('on')
        } else if (attached.kind === 'ass') {
          setCaptions('ass')
        }
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
  }, [itemId, nativeShell, title])

  useEffect(() => {
    if (!nativeActive || !nativeRequest.current) return
    let cancelled = false
    let detachSession = () => {}
    const request = nativeRequest.current
    nativeClock.current = {
      seconds: (request.startPositionMs ?? 0) / 1000,
      paused: false,
    }
    setCurrentSeconds(nativeClock.current.seconds)
    nativeSeenRunning.current = false
    void (async () => {
      try {
        nativeStarted.current = true
        await playNatively(request.url, {
          title: request.title,
          startPositionMs: request.startPositionMs,
          userAgent: request.userAgent,
          subtitle: request.subtitle,
        })
        if (cancelled) return
        setPlaying(true)
        detachSession = attachNativePlaybackSession(
          request.sessionId,
          () => ({
            positionMs: Math.max(0, Math.floor(nativeClock.current.seconds * 1000)),
            paused: nativeClock.current.paused,
          }),
          reportPlaybackSessionEvent,
        )
      } catch (cause) {
        if (!cancelled) {
          setError(cause instanceof Error ? cause.message : '系统播放器未能打开这路流。')
        }
      }
    })()
    const poll = window.setInterval(() => {
      void nativeStatus().then((status) => {
        if (!status || cancelled) return
        if (status.running) nativeSeenRunning.current = true
        else if (handoffEnded(nativeSeenRunning.current, status.running)) {
          onCloseRef.current()
          return
        }
        const wasPaused = nativeClock.current.paused
        nativeClock.current = { seconds: status.time, paused: status.paused }
        setPlaying(!status.paused)
        setCurrentSeconds(status.time)
        setDurationSeconds(status.duration)
        if (wasPaused !== status.paused && request.sessionId) {
          void reportPlaybackSessionEvent(
            request.sessionId,
            'progress',
            Math.max(0, Math.floor(status.time * 1000)),
            status.paused,
          )
        }
      })
    }, 500)
    return () => {
      cancelled = true
      window.clearInterval(poll)
      detachSession()
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
    if (!stage) return
    const onWheel = (event: WheelEvent) => {
      if ((event.target as HTMLElement | null)?.closest('input, select, button, label')) return
      event.preventDefault()
      revealChrome(false, true)
      applyZoom(stepPictureZoom(pictureZoom, event.deltaY > 0 ? -1 : 1))
    }
    stage.addEventListener('wheel', onWheel, { passive: false })
    return () => stage.removeEventListener('wheel', onWheel)
  }, [applyZoom, pictureZoom, revealChrome])

  const seekProgress = durationSeconds > 0 ? Math.min(1, currentSeconds / durationSeconds) : 0
  const shellClass = [
    'library-player',
    chromeVisible || !playing || Boolean(error) ? 'chrome-visible' : 'chrome-hidden',
    playing ? 'is-playing' : 'is-paused',
  ].filter(Boolean).join(' ')

  if (nativeShell) {
    return (
      <div
        aria-labelledby="library-player-title"
        aria-modal="true"
        className="library-player is-handoff"
        role="dialog"
      >
        <div className="library-player-handoff">
          <h2 id="library-player-title">{title}</h2>
          {error ? (
            <p className="library-player-handoff-state" role="alert">{error}</p>
          ) : (
            <p className="library-player-handoff-state" role="status">
              {nativeActive ? handoffStatusText(playing, currentSeconds, durationSeconds) : '正在准备播放…'}
            </p>
          )}
          <p className="library-player-handoff-hint">
            画面在 mpv 窗口里，全屏、进度、音轨和字幕都用 mpv 自己的控制。
          </p>
          {subtitleHint && !error ? (
            <p className="library-player-handoff-hint" role="status">{subtitleHint}</p>
          ) : null}
          <div className="library-player-handoff-actions">
            {nativeActive && !error ? (
              <button className="primary-action" onClick={togglePlayback} type="button">
                {playing ? <Pause size={17} /> : <Play size={17} />}
                {playing ? '暂停' : '继续'}
              </button>
            ) : null}
            <button className="secondary-action" onClick={onClose} type="button">
              <X size={17} />
              停止播放
            </button>
            {onNextEpisode && nextEpisodeLabel ? (
              <button className="secondary-action" onClick={onNextEpisode} type="button">
                <SkipForward size={16} />
                下一集 {nextEpisodeLabel}
              </button>
            ) : null}
            <a className="secondary-action" href={externalUrl} rel="noreferrer" target="_blank">在 Emby 打开</a>
          </div>
        </div>
      </div>
    )
  }

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
          ref={toolbarRef}
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

        <div
          className={`library-player-picture ${playerAspectClassName(aspect)}`}
          style={{ '--picture-zoom': String(pictureZoom) } as CSSProperties}
        >
          <video
            ref={videoRef}
            className="library-player-video"
            controls={false}
            onClick={onSurfaceClick}
            onDoubleClick={onSurfaceDoubleClick}
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

        {!playing && !error ? (
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

        {subtitleHint && !error ? (
          <p className="library-player-note" role="status">{subtitleHint}</p>
        ) : null}

        {captions === 'ass' ? (
          <p className="library-player-note" role="status">
            当前是 ASS 字幕，浏览器无法渲染。
            <a href={externalUrl} rel="noreferrer" target="_blank">在 Emby 打开</a>
          </p>
        ) : null}

        {silentAudio && !error ? (
          <p className="library-player-note" role="status">
            检测到有画面但浏览器未解码出声音。自己的库是直出原片，DTS / TrueHD / 部分 EAC3 音轨在 Chrome 等浏览器里常会静音。
            <a href={externalUrl} rel="noreferrer" target="_blank">在 Emby 打开</a>
          </p>
        ) : null}

        <div
          ref={chromeBarRef}
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
                if (videoRef.current) videoRef.current.currentTime = next
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
                  if (videoRef.current) videoRef.current.playbackRate = next
                  setRate(next)
                }}
                value={String(rate)}
              >
                {playbackRates.map((option) => (
                  <option key={option} value={option}>{option === 1 ? '1×' : `${option}×`}</option>
                ))}
              </select>
            </label>

            {captions === 'on' || captions === 'off' ? (
              <button
                aria-label={captions === 'off' ? '字幕关' : '字幕开'}
                aria-pressed={captions !== 'off'}
                className={captions === 'off' ? 'library-player-icon-action' : 'library-player-icon-action active'}
                onClick={toggleCaptions}
                type="button"
              >
                <Captions size={18} />
                <span aria-hidden="true">{captions === 'off' ? '字幕关' : '字幕开'}</span>
              </button>
            ) : (
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
            )}

            {pipAvailable ? (
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
              aria-pressed={webFullscreen}
              className="library-player-icon-action"
              onClick={togglePresentation}
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
