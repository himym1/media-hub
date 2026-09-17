import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  canCapturePages,
  captureFileName,
  captureItemLabel,
  isCapturablePageUrl,
  pickAutoCaptureItem,
  runPageCapture,
  uploadPageCapture,
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

  it('runs capture through Tauri', async () => {
    const invoke = vi.fn(async () => ({
      kind: 'hls',
      path: '/tmp/media-hub-capture.ts',
      share_import_url: null,
    }))
    ;(globalThis as unknown as { __TAURI_INTERNALS__: { invoke: typeof invoke } }).__TAURI_INTERNALS__ = { invoke }
    await expect(runPageCapture('site.example/watch')).resolves.toMatchObject({ kind: 'hls' })
    expect(invoke).toHaveBeenCalledWith('run_page_capture', { url: 'https://site.example/watch' })
  })

  it('falls back to list-and-download when the auto command is missing', async () => {
    const invoke = vi.fn(async (cmd: string) => {
      if (cmd === 'run_page_capture') throw new Error('Command run_page_capture not found')
      if (cmd === 'list_page_capture') {
        return {
          items: [
            { url: 'https://cdn.example/ad.mp4', kind: 'file' },
            { url: 'https://cdn.example/master.m3u8', kind: 'hls' },
          ],
        }
      }
      if (cmd === 'download_page_capture') {
        return { kind: 'hls', path: '/tmp/media-hub-capture.ts', share_import_url: null }
      }
      return undefined
    })
    ;(globalThis as unknown as { __TAURI_INTERNALS__: { invoke: typeof invoke } }).__TAURI_INTERNALS__ = { invoke }
    await expect(runPageCapture('https://site.example/watch')).resolves.toMatchObject({ kind: 'hls' })
    expect(invoke).toHaveBeenCalledWith('download_page_capture', { url: 'https://cdn.example/master.m3u8' })
  })

  it('labels items by host and file name', () => {
    expect(captureItemLabel({
      url: 'https://cdn.example/hls/index.m3u8?token=1',
      kind: 'hls',
    })).toBe('cdn.example · index.m3u8')
  })

  it('picks the latest hls over files', () => {
    expect(pickAutoCaptureItem([
      { url: 'https://cdn.example/ad.mp4', kind: 'file' },
      { url: 'https://cdn.example/master.m3u8', kind: 'hls' },
      { url: 'https://cdn.example/media.m3u8', kind: 'hls' },
    ])?.url).toBe('https://cdn.example/media.m3u8')
  })

  it('reads the capture file name from local paths', () => {
    expect(captureFileName('/Users/me/Downloads/MediaHubCapture/clip.ts')).toBe('clip.ts')
    expect(captureFileName('C:\\Temp\\MediaHubCapture\\clip.ts')).toBe('clip.ts')
  })

  it('uploads the captured file through Tauri', async () => {
    const invoke = vi.fn(async () => undefined)
    ;(globalThis as unknown as { __TAURI_INTERNALS__: { invoke: typeof invoke } }).__TAURI_INTERNALS__ = { invoke }
    const ticket = {
      destinationId: '9001',
      filename: 'clip.ts',
      title: 'clip',
      target: 'U_1_9001',
      host: 'https://bucket.oss-cn-shenzhen.aliyuncs.com',
      object: 'obj',
      accessid: 'id',
      policy: 'p',
      signature: 's',
      callback: 'cb',
    }
    await uploadPageCapture('/tmp/clip.ts', ticket)
    expect(invoke).toHaveBeenCalledWith('upload_page_capture', { path: '/tmp/clip.ts', ticket })
  })
})
