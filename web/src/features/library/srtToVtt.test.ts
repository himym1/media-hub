import { describe, expect, it } from 'vitest'
import { isAssSubtitle, srtToVtt, subtitleFileName } from './srtToVtt'

describe('srtToVtt', () => {
  it('converts SRT timestamps and adds a WEBVTT header', () => {
    expect(srtToVtt('1\n00:00:01,000 --> 00:00:04,000\nHello')).toBe(
      'WEBVTT\n\n1\n00:00:01.000 --> 00:00:04.000\nHello\n',
    )
  })

  it('leaves existing WebVTT alone', () => {
    expect(srtToVtt('WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nHi')).toBe(
      'WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nHi\n',
    )
  })

  it('detects ASS sidecars', () => {
    expect(isAssSubtitle('text/x-ssa', 'chi.srt')).toBe(true)
    expect(isAssSubtitle('application/x-subrip', 'chi.ass')).toBe(true)
    expect(isAssSubtitle('application/x-subrip', 'chi.srt')).toBe(false)
  })

  it('reads a Content-Disposition file name', () => {
    expect(subtitleFileName('attachment; filename="chi.srt"')).toBe('chi.srt')
    expect(subtitleFileName("attachment; filename*=UTF-8''%E4%B8%AD%E6%96%87.srt")).toBe('中文.srt')
  })
})
