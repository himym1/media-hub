import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError, getLatestDesktopRelease, type DesktopRelease } from '../api/mediaHub'
import {
  desktopAppPlatform,
  desktopAppVersion,
  desktopInstallerPath,
  desktopInstallerFileName,
  desktopNativeInstallReady,
  downloadDesktopInstaller,
  startDesktopInstallerDownload,
  desktopUpdateErrorMessage,
  desktopUpdateRequired,
  desktopVersionCode,
  formatDesktopUpdateSize,
  installDesktopUpdate,
  isDesktopShell,
  isDesktopUpdateCancelled,
  newerDesktopRelease,
} from './desktopUpdate'

afterEach(() => {
  delete (globalThis as { __TAURI_INTERNALS__?: unknown }).__TAURI_INTERNALS__
})

const release: DesktopRelease = {
  versionCode: 20039,
  versionName: '0.20.39',
  minimumSupportedVersionCode: 20010,
  sha256: 'a'.repeat(64),
  sizeBytes: 18 * 1024 * 1024,
  publishedAt: '2026-09-14T08:00:00Z',
  notes: 'Media Hub 0.20.39',
  downloadPath: '/api/v1/client/desktop/releases/20039/installer',
}

describe('desktopUpdate', () => {
  it('is off in the browser', () => {
    expect(isDesktopShell()).toBe(false)
  })

  it('uses the Android versionCode formula', () => {
    expect(desktopVersionCode('0.20.38')).toBe(20038)
    expect(desktopVersionCode('1.2.3-dev')).toBe(1002003)
    expect(desktopVersionCode('bad')).toBeNull()
  })

  it('only prompts when the published code is newer', () => {
    expect(newerDesktopRelease(20038, release)).toEqual(release)
    expect(newerDesktopRelease(20039, release)).toBeNull()
    expect(desktopUpdateRequired(20009, release)).toBe(true)
    expect(desktopUpdateRequired(20010, release)).toBe(false)
  })

  it('keeps Windows on the download path until the silent updater ships', () => {
    expect(desktopNativeInstallReady('0.20.48', 'windows')).toBe(false)
    expect(desktopNativeInstallReady('0.20.50', 'windows')).toBe(true)
    expect(desktopNativeInstallReady('0.20.48', 'darwin')).toBe(true)
    expect(desktopNativeInstallReady(null, 'windows')).toBe(false)
  })

  it('formats installer size and hides raw URLs', () => {
    expect(formatDesktopUpdateSize(release.sizeBytes)).toBe('18.0 MB')
    expect(desktopUpdateErrorMessage(new ApiError(404, 'desktop_release_unavailable', '当前没有可用的桌面更新'))).toBe('当前没有可用的桌面更新')
    expect(desktopUpdateErrorMessage('https://media.himym.us.ci/secret')).toBe('桌面更新失败')
  })

  it('does not reuse a cached latest desktop release', async () => {
    const fetch = vi.fn(async () => new Response(JSON.stringify(release), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    }))
    vi.stubGlobal('fetch', fetch)
    await expect(getLatestDesktopRelease('darwin')).resolves.toEqual(release)
    expect(fetch).toHaveBeenCalledWith(
      '/api/v1/client/desktop/releases/latest?platform=darwin',
      expect.objectContaining({ cache: 'no-store', credentials: 'same-origin' }),
    )
    vi.unstubAllGlobals()
  })

  it('reads the packaged app version from Tauri', async () => {
    const invoke = vi.fn(async () => '0.20.38')
    ;(globalThis as unknown as { __TAURI_INTERNALS__: { invoke: typeof invoke } }).__TAURI_INTERNALS__ = { invoke }
    await expect(desktopAppVersion()).resolves.toBe('0.20.38')
    expect(invoke).toHaveBeenCalledWith('desktop_app_version', {})
  })

  it('treats a missing version command as an older desktop shell', async () => {
    const invoke = vi.fn(async () => {
      throw new Error('command desktop_app_version not found')
    })
    ;(globalThis as unknown as { __TAURI_INTERNALS__: { invoke: typeof invoke } }).__TAURI_INTERNALS__ = { invoke }
    await expect(desktopAppVersion()).resolves.toBeNull()
  })

  it('starts a WebView download without the download attribute', () => {
    const clicks: Array<{ href: string; download: string }> = []
    const origin = 'https://media.himym.us.ci'
    const anchor = {
      href: '',
      rel: '',
      click() {
        clicks.push({ href: this.href, download: '' })
      },
      remove() {},
    }
    vi.stubGlobal('window', { location: { origin } })
    vi.stubGlobal('document', {
      createElement: (tag: string) => {
        expect(tag).toBe('a')
        return anchor
      },
      body: { append: vi.fn() },
    })
    expect(startDesktopInstallerDownload(release)).toBe('media-hub-20039.exe')
    expect(clicks).toEqual([{
      href: `${origin}/api/v1/client/desktop/releases/20039/installer`,
      download: '',
    }])
    expect(() => startDesktopInstallerDownload({ ...release, downloadPath: '/tmp/setup.exe' })).toThrow('更新地址无效')
    vi.unstubAllGlobals()
  })

  it('saves the private installer through the file picker', async () => {
    const writable = { write: vi.fn(), close: vi.fn() }
    const picker = vi.fn(async () => ({ createWritable: async () => writable }))
    const bytes = new Uint8Array([1, 2, 3, 4])
    vi.stubGlobal('showSaveFilePicker', picker)
    vi.stubGlobal('fetch', vi.fn(async () => ({
      ok: true,
      status: 200,
      blob: async () => new Blob([bytes]),
    })))
    const digest = await crypto.subtle.digest('SHA-256', bytes)
    const sha256 = [...new Uint8Array(digest)].map((byte) => byte.toString(16).padStart(2, '0')).join('')
    await downloadDesktopInstaller({ ...release, sha256, sizeBytes: bytes.byteLength })
    expect(picker).toHaveBeenCalled()
    expect(writable.write).toHaveBeenCalled()
    expect(writable.close).toHaveBeenCalled()
    await expect(downloadDesktopInstaller({ ...release, downloadPath: '/tmp/setup.exe' })).rejects.toThrow('更新地址无效')
    expect(desktopInstallerFileName({ ...release, downloadPath: desktopInstallerPath(20039, 'darwin') })).toBe('media-hub-20039.dmg')
    expect(isDesktopUpdateCancelled(new DOMException('cancelled', 'AbortError'))).toBe(true)
    vi.unstubAllGlobals()
  })

  it('reads darwin from the desktop platform command', async () => {
    const invoke = vi.fn(async () => 'darwin')
    ;(globalThis as unknown as { __TAURI_INTERNALS__: { invoke: typeof invoke } }).__TAURI_INTERNALS__ = { invoke }
    await expect(desktopAppPlatform()).resolves.toBe('darwin')
  })

  it('installs only the matching private download path', async () => {
    const invoke = vi.fn(async () => undefined)
    ;(globalThis as unknown as { __TAURI_INTERNALS__: { invoke: typeof invoke } }).__TAURI_INTERNALS__ = { invoke }
    await installDesktopUpdate(release)
    expect(invoke).toHaveBeenCalledWith('install_desktop_update', {
      downloadPath: release.downloadPath,
      sha256: release.sha256,
      sizeBytes: release.sizeBytes,
    })
    await expect(installDesktopUpdate({ ...release, downloadPath: '/api/v1/client/android/releases/20039/apk' })).rejects.toThrow('更新地址无效')
  })
})
