import { describe, expect, it } from 'vitest'
import { looksLikeSilentDirectPlay } from './silentAudio'

function videoStub(overrides: Record<string, unknown> = {}) {
  return {
    paused: false,
    ended: false,
    readyState: 2,
    muted: false,
    volume: 1,
    currentTime: 4,
    ...overrides,
  } as unknown as HTMLVideoElement
}

describe('looksLikeSilentDirectPlay', () => {
  it('returns false before enough playback progress', () => {
    expect(looksLikeSilentDirectPlay(videoStub({
      currentTime: 1,
      webkitAudioDecodedByteCount: 0,
      webkitVideoDecodedByteCount: 1000,
    }))).toBe(false)
  })

  it('detects chrome-style silent decode counters', () => {
    expect(looksLikeSilentDirectPlay(videoStub({
      webkitAudioDecodedByteCount: 0,
      webkitVideoDecodedByteCount: 2048,
    }))).toBe(true)
  })

  it('returns false when audio bytes are flowing', () => {
    expect(looksLikeSilentDirectPlay(videoStub({
      webkitAudioDecodedByteCount: 512,
      webkitVideoDecodedByteCount: 2048,
    }))).toBe(false)
  })

  it('ignores muted playback', () => {
    expect(looksLikeSilentDirectPlay(videoStub({
      muted: true,
      webkitAudioDecodedByteCount: 0,
      webkitVideoDecodedByteCount: 2048,
    }))).toBe(false)
  })
})
