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

const desktopUpdateStorageKey = 'media-hub.desktop-update'

export type StoredDesktopUpdate = {
  dismissedCode?: number
  downloadedCode?: number
}

export type DesktopUpdateDecision = {
  release: DesktopRelease | null
  required: boolean
  pendingRelaunch: boolean
}

function desktopUpdateStorage(): Storage | null {
  try {
    return globalThis.localStorage
  } catch {
    return null
  }
}

export function readStoredDesktopUpdate(): StoredDesktopUpdate {
  const raw = desktopUpdateStorage()?.getItem(desktopUpdateStorageKey)
  if (!raw) return {}
  try {
    const value = JSON.parse(raw) as StoredDesktopUpdate
    return {
      dismissedCode: Number.isInteger(value.dismissedCode) ? value.dismissedCode : undefined,
      downloadedCode: Number.isInteger(value.downloadedCode) ? value.downloadedCode : undefined,
    }
  } catch {
    return {}
  }
}

function writeStoredDesktopUpdate(value: StoredDesktopUpdate) {
  const storage = desktopUpdateStorage()
  if (!storage) return
  storage.setItem(desktopUpdateStorageKey, JSON.stringify(value))
}

export function rememberDownloadedDesktopRelease(versionCode: number) {
  writeStoredDesktopUpdate({ ...readStoredDesktopUpdate(), downloadedCode: versionCode })
}

export function rememberDismissedDesktopRelease(versionCode: number) {
  writeStoredDesktopUpdate({ ...readStoredDesktopUpdate(), dismissedCode: versionCode })
}

export function clearStoredDesktopUpdate() {
  desktopUpdateStorage()?.removeItem(desktopUpdateStorageKey)
}

export function resolveDesktopUpdate(
  currentVersion: string | null,
  latest: DesktopRelease,
  stored: StoredDesktopUpdate = readStoredDesktopUpdate(),
): DesktopUpdateDecision {
  if (currentVersion == null) {
    if (stored.dismissedCode === latest.versionCode) {
      return { release: null, required: false, pendingRelaunch: false }
    }
    if (stored.downloadedCode === latest.versionCode) {
      return { release: latest, required: false, pendingRelaunch: true }
    }
    return { release: latest, required: false, pendingRelaunch: false }
  }
  const currentCode = desktopVersionCode(currentVersion)
  if (currentCode == null) {
    return { release: null, required: false, pendingRelaunch: false }
  }
  if (currentCode >= latest.versionCode) {
    clearStoredDesktopUpdate()
    return { release: null, required: false, pendingRelaunch: false }
  }
  const next = newerDesktopRelease(currentCode, latest)
  return {
    release: next,
    required: Boolean(next && desktopUpdateRequired(currentCode, latest)),
    pendingRelaunch: false,
  }
}

export function desktopUpdateRequired(currentVersionCode: number, latest: DesktopRelease) {
  return currentVersionCode < latest.minimumSupportedVersionCode
}

export function desktopNativeInstallReady(version: string | null, platform: DesktopPlatform | null) {
  void version
  void platform
  // In-app replace is not reliable yet: Windows silent NSIS often never
  // finishes, and macOS only opens the DMG. Keep one download action.
  return false
}

export function formatDesktopUpdateSize(bytes: number) {
  if (bytes <= 0) return '未知大小'
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

export function desktopReleaseBlurb(release: DesktopRelease): string | null {
  const notes = release.notes.trim()
  if (!notes || notes === `Media Hub ${release.versionName}`) return null
  return notes
}

export function desktopUpdateHeadline(release: DesktopRelease, pendingRelaunch: boolean) {
  return pendingRelaunch ? `${release.versionName} 已就绪` : `桌面端 ${release.versionName}`
}

const darwinGatekeeperHint =
  '若提示已损坏，终端执行 xattr -cr "/Applications/Media Hub.app"'

export function desktopUpdateBody(
  release: DesktopRelease,
  pendingRelaunch: boolean,
  nativeInstall: boolean,
) {
  if (pendingRelaunch) {
    const reopen = '请完全退出，再从「应用程序」打开。只关窗口会继续用旧版。'
    return desktopPlatformFromPath(release.downloadPath) === 'darwin'
      ? `${reopen}${darwinGatekeeperHint}`
      : reopen
  }
  const size = formatDesktopUpdateSize(release.sizeBytes)
  const blurb = desktopReleaseBlurb(release)
  const lead = blurb ? `${blurb} · 约 ${size}` : `约 ${size}`
  if (desktopPlatformFromPath(release.downloadPath) === 'darwin') {
    const install = nativeInstall
      ? `${lead}。装好后完全退出，从「应用程序」打开。`
      : `${lead}。拖到「应用程序」后完全退出再打开。`
    return `${install}${darwinGatekeeperHint}`
  }
  return nativeInstall ? `${lead}。安装后会重新打开。` : `${lead}。退出后再打开安装包。`
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

export function startDesktopInstallerDownload(release: DesktopRelease) {
  const fileName = desktopInstallerFileName(release)
  const href = new URL(release.downloadPath, window.location.origin).toString()
  if (!href.startsWith(`${window.location.origin}/api/v1/client/desktop/releases/`)) {
    throw new Error('更新地址无效')
  }
  const anchor = document.createElement('a')
  anchor.href = href
  anchor.rel = 'noopener'
  // WebView2 swallows `<a download>` and never raises Tauri's on_download.
  document.body.append(anchor)
  anchor.click()
  anchor.remove()
  rememberDownloadedDesktopRelease(release.versionCode)
  return fileName
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
    rememberDownloadedDesktopRelease(release.versionCode)
    return
  }
  clickDownloadBlob(await fetchInstallerBlob(release), fileName)
  rememberDownloadedDesktopRelease(release.versionCode)
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
  rememberDownloadedDesktopRelease(release.versionCode)
}
