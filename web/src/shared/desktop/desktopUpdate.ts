import { ApiError, type DesktopRelease } from '../api/mediaHub'

type TauriInvoke = (cmd: string, args?: Record<string, unknown>) => Promise<unknown>

function tauriInvoke(): TauriInvoke | null {
  const internals = (globalThis as { __TAURI_INTERNALS__?: { invoke?: TauriInvoke } }).__TAURI_INTERNALS__
  return typeof internals?.invoke === 'function' ? internals.invoke.bind(internals) : null
}

export function isDesktopShell() {
  return tauriInvoke() !== null
}

export type DesktopPlatform = 'windows' | 'darwin'

export function desktopInstallerPath(versionCode: number, platform: DesktopPlatform) {
  return platform === 'darwin'
    ? `/api/v1/client/desktop/releases/${versionCode}/dmg`
    : `/api/v1/client/desktop/releases/${versionCode}/installer`
}

export function desktopPlatformFromPath(downloadPath: string): DesktopPlatform | null {
  if (downloadPath.endsWith('/dmg')) return 'darwin'
  if (downloadPath.endsWith('/installer')) return 'windows'
  return null
}

export function desktopVersionCode(versionName: string): number | null {
  const match = /^(\d+)\.(\d+)\.(\d+)/.exec(versionName.trim())
  if (!match) return null
  return Number(match[1]) * 1_000_000 + Number(match[2]) * 1_000 + Number(match[3])
}

export function newerDesktopRelease(currentVersionCode: number, latest: DesktopRelease): DesktopRelease | null {
  return latest.versionCode > currentVersionCode ? latest : null
}

export function desktopUpdateRequired(currentVersionCode: number, latest: DesktopRelease) {
  return currentVersionCode < latest.minimumSupportedVersionCode
}

export function desktopNativeInstallReady(version: string | null, platform: DesktopPlatform | null) {
  if (!version || !platform) return false
  if (platform === 'darwin') return desktopVersionCode(version) != null
  const code = desktopVersionCode(version)
  return code != null && code >= 20_050
}

export function formatDesktopUpdateSize(bytes: number) {
  if (bytes <= 0) return '未知大小'
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

export function desktopUpdateErrorMessage(cause: unknown) {
  if (cause instanceof ApiError) {
    if (cause.status === 404 || cause.code === 'desktop_release_unavailable') {
      return '当前没有可用的桌面更新'
    }
    return cause.message
  }
  const raw = typeof cause === 'string'
    ? cause
    : cause instanceof Error
      ? cause.message
      : ''
  if (!raw || /https?:\/\//i.test(raw)) {
    return '桌面更新失败'
  }
  return raw
}

async function invokeDesktop(cmd: string, args: Record<string, unknown> = {}) {
  const invoke = tauriInvoke()
  if (!invoke) throw new Error('当前不是桌面应用')
  try {
    return await invoke(cmd, args)
  } catch (cause) {
    throw new Error(desktopUpdateErrorMessage(cause))
  }
}

export async function desktopAppPlatform(): Promise<DesktopPlatform | null> {
  if (!isDesktopShell()) return null
  try {
    const value = await invokeDesktop('desktop_app_platform')
    if (value === 'darwin' || value === 'windows') return value
  } catch {
    // Older desktop builds only expose invoke, not the platform command.
  }
  if (typeof navigator === 'undefined') return null
  if (/Mac/i.test(navigator.userAgent)) return 'darwin'
  if (/Windows/i.test(navigator.userAgent)) return 'windows'
  return null
}

export async function desktopAppVersion() {
  try {
    const value = await invokeDesktop('desktop_app_version')
    if (typeof value !== 'string' || !value.trim()) {
      return null
    }
    return value.trim()
  } catch {
    return null
  }
}

export function desktopInstallerFileName(release: DesktopRelease) {
  const platform = desktopPlatformFromPath(release.downloadPath)
  if (!platform || release.downloadPath !== desktopInstallerPath(release.versionCode, platform)) {
    throw new Error('更新地址无效')
  }
  return `media-hub-${release.versionCode}.${platform === 'darwin' ? 'dmg' : 'exe'}`
}

export function isDesktopUpdateCancelled(cause: unknown) {
  return cause instanceof DOMException && cause.name === 'AbortError'
}

type SaveFileHandle = {
  createWritable: () => Promise<{ write: (data: Blob) => Promise<void>; close: () => Promise<void> }>
}

function saveFilePicker() {
  const picker = (globalThis as {
    showSaveFilePicker?: (options: {
      suggestedName: string
      types: Array<{ description: string; accept: Record<string, string[]> }>
    }) => Promise<SaveFileHandle>
  }).showSaveFilePicker
  return typeof picker === 'function' ? picker.bind(globalThis) : null
}

async function fetchInstallerBlob(release: DesktopRelease) {
  const response = await fetch(release.downloadPath, {
    credentials: 'same-origin',
    headers: { Accept: 'application/octet-stream' },
  })
  if (response.status === 401) {
    throw new Error('登录已过期，请重新登录后再更新。')
  }
  if (!response.ok) {
    throw new Error('无法下载桌面更新')
  }
  const blob = await response.blob()
  if (blob.size !== release.sizeBytes || !(await installerChecksumMatches(blob, release.sha256))) {
    throw new Error('更新包校验失败')
  }
  return blob
}

async function installerChecksumMatches(blob: Blob, expected: string) {
  if (!globalThis.crypto?.subtle) {
    return blob.size > 0
  }
  const digest = await crypto.subtle.digest('SHA-256', await blob.arrayBuffer())
  const hex = [...new Uint8Array(digest)].map((byte) => byte.toString(16).padStart(2, '0')).join('')
  return hex === expected.trim().toLowerCase()
}

function clickDownloadBlob(blob: Blob, fileName: string) {
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = fileName
  anchor.rel = 'noopener'
  document.body.append(anchor)
  anchor.click()
  anchor.remove()
  window.setTimeout(() => URL.revokeObjectURL(url), 1000)
}

export async function downloadDesktopInstaller(release: DesktopRelease) {
  const fileName = desktopInstallerFileName(release)
  const picker = saveFilePicker()
  if (picker) {
    const handle = await picker({
      suggestedName: fileName,
      types: [{ description: 'Media Hub', accept: { 'application/octet-stream': [`.${fileName.split('.').pop()}`] } }],
    })
    const blob = await fetchInstallerBlob(release)
    const writable = await handle.createWritable()
    try {
      await writable.write(blob)
    } finally {
      await writable.close()
    }
    return
  }
  clickDownloadBlob(await fetchInstallerBlob(release), fileName)
}

export async function installDesktopUpdate(release: DesktopRelease) {
  const platform = desktopPlatformFromPath(release.downloadPath)
  if (!platform || release.downloadPath !== desktopInstallerPath(release.versionCode, platform)) {
    throw new Error('更新地址无效')
  }
  await invokeDesktop('install_desktop_update', {
    downloadPath: release.downloadPath,
    sha256: release.sha256,
    sizeBytes: release.sizeBytes,
  })
}
