import { describe, expect, it } from 'vitest'
import { isHlsStream } from './isHlsStream'

describe('isHlsStream', () => {
  it('detects m3u8 pathnames and rejects progressive urls', () => {
    expect(isHlsStream('https://cdn.example/movie.M3U8?token=1')).toBe(true)
    expect(isHlsStream('https://cdn.example/movie.mp4')).toBe(false)
    expect(isHlsStream('not a url')).toBe(false)
  })
})
