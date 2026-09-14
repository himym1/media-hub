import type { LocalSubtitle } from '../../shared/api/mediaHub'
import { formatPlaybackClock } from './libraryPlayback'
import { isAssSubtitle, srtToVtt } from './srtToVtt'

export type SubtitleAttachResult =
  | { kind: 'track'; url: string }
  | { kind: 'ass' }
  | { kind: 'none' }

export function subtitleAttachResult(subtitle: LocalSubtitle | null): SubtitleAttachResult {
  if (!subtitle || subtitle.bytes.byteLength === 0) return { kind: 'none' }
  if (isAssSubtitle(subtitle.contentType, subtitle.fileName)) return { kind: 'ass' }
  const text = new TextDecoder('utf-8', { fatal: false }).decode(subtitle.bytes)
  const url = URL.createObjectURL(new Blob([srtToVtt(text)], { type: 'text/vtt' }))
  return { kind: 'track', url }
}

export function attachSubtitleTrack(video: HTMLVideoElement, src: string) {
  const track = document.createElement('track')
  track.kind = 'subtitles'
  track.label = '中文'
  track.srclang = 'zh'
  track.src = src
  track.default = true
  video.appendChild(track)
  return () => {
    track.remove()
  }
}

export function setSubtitleMode(video: HTMLVideoElement, showing: boolean) {
  for (const track of video.textTracks) {
    track.mode = showing ? 'showing' : 'hidden'
  }
}

export function firstSubtitleStartSeconds(bytes: ArrayBuffer): number | null {
  if (bytes.byteLength === 0) return null
  const text = new TextDecoder('utf-8', { fatal: false }).decode(bytes)
  const match = /(\d{2}):(\d{2}):(\d{2})[,.]\d{1,3}\s+-->/.exec(text)
  if (!match) return null
  return Number(match[1]) * 3600 + Number(match[2]) * 60 + Number(match[3])
}

export function subtitleStartHint(bytes: ArrayBuffer): string | null {
  const start = firstSubtitleStartSeconds(bytes)
  if (start == null || start < 60) return null
  return `中文字幕从 ${formatPlaybackClock(start)} 才开始，片头没有字`
}
