import { describe, expect, it } from 'vitest'
import { subtitleAttachResult } from './librarySubtitle'

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
})
