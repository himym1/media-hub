import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError, type DesktopRelease } from '../api/mediaHub'
import {
  desktopAppPlatform,
  desktopAppVersion,
  desktopInstallerPath,
  downloadDesktopInstaller,
  desktopUpdateErrorMessage,
  desktopUpdateRequired,
  desktopVersionCode,
  formatDesktopUpdateSize,
  installDesktopUpdate,
  isDesktopShell,
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

  it('formats installer size and hides raw URLs', () => {
    expect(formatDesktopUpdateSize(release.sizeBytes)).toBe('18.0 MB')
    expect(desktopUpdateErrorMessage(new ApiError(404, 'desktop_release_unavailable', '当前没有可用的桌面更新'))).toBe('当前没有可用的桌面更新')
    expect(desktopUpdateErrorMessage('https://media.himym.us.ci/secret')).toBe('桌面更新失败')
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

  it('opens the private installer path for older desktop shells', () => {
    const anchor = { href: '', download: '', rel: '', click: vi.fn(), remove: vi.fn() }
    const body = { append: vi.fn() }
    vi.stubGlobal('document', { body, createElement: () => anchor })
    downloadDesktopInstaller(release)
    expect(anchor.href).toBe(release.downloadPath)
    expect(anchor.download).toBe('media-hub-20039.exe')
    expect(anchor.click).toHaveBeenCalled()
    expect(() => downloadDesktopInstaller({ ...release, downloadPath: '/tmp/setup.exe' })).toThrow('更新地址无效')
    const mac = { ...release, downloadPath: desktopInstallerPath(20039, 'darwin') }
    downloadDesktopInstaller(mac)
    expect(anchor.href).toBe(mac.downloadPath)
    expect(anchor.download).toBe('media-hub-20039.dmg')
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
