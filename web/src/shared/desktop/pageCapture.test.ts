import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  canCapturePages,
  captureItemLabel,
  isCapturablePageUrl,
  listPageCapture,
  openPageCapture,
} from './pageCapture'

afterEach(() => {
  delete (globalThis as { __TAURI_INTERNALS__?: unknown }).__TAURI_INTERNALS__
})

describe('pageCapture', () => {
  it('is off in the browser', () => {
    expect(canCapturePages()).toBe(false)
  })

  it('accepts ordinary web pages', () => {
    expect(isCapturablePageUrl('https://site.example/watch?id=1')).toBe(true)
    expect(isCapturablePageUrl('site.example/watch')).toBe(true)
    expect(isCapturablePageUrl('javascript:alert(1)')).toBe(false)
    expect(isCapturablePageUrl('https://user:pass@site.example/')).toBe(false)
  })

  it('opens and lists through Tauri', async () => {
    const invoke = vi.fn(async (cmd: string) => {
      if (cmd === 'list_page_capture') {
        return { page: 'https://site.example/watch', items: [{ url: 'https://cdn.example/a.m3u8', kind: 'hls' }] }
      }
      return undefined
    })
    ;(globalThis as unknown as { __TAURI_INTERNALS__: { invoke: typeof invoke } }).__TAURI_INTERNALS__ = { invoke }
    await openPageCapture('https://site.example/watch')
    expect(invoke).toHaveBeenCalledWith('open_page_capture', { url: 'https://site.example/watch' })
    const snapshot = await listPageCapture()
    expect(snapshot.items[0]?.kind).toBe('hls')
  })

  it('labels items by host and file name', () => {
    expect(captureItemLabel({
      url: 'https://cdn.example/hls/index.m3u8?token=1',
      kind: 'hls',
    })).toBe('cdn.example · index.m3u8')
  })
})
