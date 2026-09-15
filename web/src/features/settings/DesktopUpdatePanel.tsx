import { useState, type ReactNode } from 'react'
import { Download, RefreshCw } from 'lucide-react'
import type { DesktopRelease } from '../../shared/api/mediaHub'
import { desktopInstallerFileName, desktopPlatformFromPath, formatDesktopUpdateSize, startDesktopInstallerDownload } from '../../shared/desktop/desktopUpdate'
import type { DesktopUpdateState } from '../../shared/desktop/useDesktopUpdate'

export function DesktopUpdateBanner({ update }: { update: DesktopUpdateState }) {
  if (!update.available || !update.prompt) return null
  return (
    <div className="desktop-update-banner" role="status">
      <p>
        桌面端 {update.prompt.versionName} 已发布
        {update.prompt.notes ? ` · ${update.prompt.notes}` : ''}
        。{updateHint(update.nativeInstall, update.prompt.downloadPath)}，约 {formatDesktopUpdateSize(update.prompt.sizeBytes)}。
      </p>
      <div className="desktop-update-actions">
        {update.required ? null : (
          <button className="secondary-command" disabled={update.installing} onClick={update.dismiss} type="button">稍后</button>
        )}
        <UpdateAction installing={update.installing} nativeInstall={update.nativeInstall} onInstall={update.install} release={update.prompt} />
        {update.nativeInstall ? <SaveInstallerLink className="secondary-command" release={update.prompt}>保存安装包</SaveInstallerLink> : null}
      </div>
      {update.error ? <span className="form-error" role="alert">{update.error}</span> : null}
    </div>
  )
}

export function DesktopUpdateSettings({ update }: { update: DesktopUpdateState }) {
  if (!update.available) return null
  const label = update.release
    ? `发现 ${update.release.versionName}，约 ${formatDesktopUpdateSize(update.release.sizeBytes)}`
    : update.checking
      ? '正在检查更新…'
      : update.error && !update.release
        ? update.error
        : `当前已是 ${update.currentVersion || '最新版本'}`
  return (
    <section className="session-actions desktop-update-settings">
      <div>
        <strong>桌面端更新</strong>
        <span>{label}</span>
      </div>
      <div className="desktop-update-actions">
        <button
          className="secondary-command"
          disabled={update.checking || update.installing}
          onClick={() => void update.check()}
          type="button"
        >
          <RefreshCw size={16} />
          {update.checking ? '正在检查' : '检查更新'}
        </button>
        {update.release && !update.nativeInstall ? (
          <SaveInstallerLink className="secondary-command" release={update.release}>
            <Download size={16} />
            下载 {update.release.versionName}
          </SaveInstallerLink>
        ) : update.release ? (
          <button
            className="secondary-command"
            disabled={update.installing}
            onClick={() => void update.install()}
            type="button"
          >
            <Download size={16} />
            {update.installing ? '正在更新…' : `更新 ${update.release.versionName}`}
          </button>
        ) : null}
        {update.release && update.nativeInstall ? (
          <SaveInstallerLink className="secondary-command" release={update.release}>保存安装包</SaveInstallerLink>
        ) : null}
      </div>
      {update.error && update.release ? <span className="form-error" role="alert">{update.error}</span> : null}
    </section>
  )
}

function UpdateAction({
  installing,
  nativeInstall,
  onInstall,
  release,
}: {
  installing: boolean
  nativeInstall: boolean
  onInstall: () => Promise<void>
  release: DesktopRelease
}) {
  if (!nativeInstall) {
    return (
      <SaveInstallerLink className="primary-action" release={release}>
        <Download size={16} />
        下载 {release.versionName}
      </SaveInstallerLink>
    )
  }
  return (
    <button className="primary-action" disabled={installing} onClick={() => void onInstall()} type="button">
      <Download size={16} />
      {installing ? '正在更新…' : `更新 ${release.versionName}`}
    </button>
  )
}

function SaveInstallerLink({
  children,
  className,
  release,
}: {
  children: ReactNode
  className: string
  release: DesktopRelease
}) {
  const [status, setStatus] = useState<string | null>(null)
  try {
    desktopInstallerFileName(release)
  } catch {
    return null
  }
  return (
    <>
      <button
        className={className}
        onClick={() => {
          try {
            const fileName = startDesktopInstallerDownload(release)
            setStatus(`已保存到「下载」文件夹，请完全退出后再打开 ${fileName}`)
          } catch (cause) {
            setStatus(cause instanceof Error ? cause.message : '无法下载桌面更新')
          }
        }}
        type="button"
      >
        {children}
      </button>
      {status ? <span className="form-hint" role="status">{status}</span> : null}
    </>
  )
}

function updateHint(nativeInstall: boolean, downloadPath: string) {
  if (desktopPlatformFromPath(downloadPath) === 'darwin') {
    return nativeInstall ? '下载后会打开安装盘，请把 Media Hub 拖到「应用程序」后重新打开' : '下载后请退出应用，打开安装盘并把 Media Hub 拖到「应用程序」'
  }
  return nativeInstall
    ? '会自动安装并重新打开，大约半分钟'
    : '请先完全退出 Media Hub，再打开约 3.1 MB 的安装包。若被拦截，点「更多信息 / 仍要运行」'
}
