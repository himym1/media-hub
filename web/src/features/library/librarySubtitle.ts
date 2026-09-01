import { isAssSubtitle, srtToVtt } from './srtToVtt'
import type { LocalSubtitle } from '../../shared/api/mediaHub'

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
