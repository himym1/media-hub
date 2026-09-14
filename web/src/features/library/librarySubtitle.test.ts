import { describe, expect, it } from 'vitest'
import { firstSubtitleStartSeconds, subtitleAttachResult, subtitleStartHint } from './librarySubtitle'

describe('subtitleAttachResult', () => {
  it('skips empty sidecars', () => {
    expect(subtitleAttachResult(null).kind).toBe('none')
    expect(subtitleAttachResult({ bytes: new ArrayBuffer(0), contentType: 'application/x-subrip', fileName: 'chi.srt' }).kind).toBe('none')
  })

  it('does not convert ASS into a browser track', () => {
    expect(subtitleAttachResult({
      bytes: new TextEncoder().encode('[Script Info]').buffer,
      contentType: 'text/x-ssa',
      fileName: 'chi.ass',
    }).kind).toBe('ass')
  })

  it('builds a VTT object URL for SRT', () => {
    const result = subtitleAttachResult({
      bytes: new TextEncoder().encode('1\n00:00:01,000 --> 00:00:02,000\nHi').buffer,
      contentType: 'application/x-subrip',
      fileName: 'chi.srt',
    })
    expect(result.kind).toBe('track')
    if (result.kind === 'track') URL.revokeObjectURL(result.url)
  })

  it('warns when the first cue is after the opening', () => {
    const bytes = new TextEncoder().encode('1\n00:04:38,120 --> 00:04:40,000\n亲爱的').buffer
    expect(firstSubtitleStartSeconds(bytes)).toBe(4 * 60 + 38)
    expect(subtitleStartHint(bytes)).toBe('中文字幕从 4:38 才开始，片头没有字')
    expect(subtitleStartHint(new TextEncoder().encode('1\n00:00:12,000 --> 00:00:13,000\nHi').buffer)).toBeNull()
  })
})
