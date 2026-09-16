import { useState, type ReactNode } from 'react'
import { Download, RefreshCw } from 'lucide-react'
import type { DesktopRelease } from '../../shared/api/mediaHub'
import { desktopInstallerFileName, desktopUpdateBody, desktopUpdateHeadline, formatDesktopUpdateSize, startDesktopInstallerDownload } from '../../shared/desktop/desktopUpdate'
import type { DesktopUpdateState } from '../../shared/desktop/useDesktopUpdate'

export function DesktopUpdateBanner({ update }: { update: DesktopUpdateState }) {
  if (!update.available || !update.prompt) return null
  return (
    <div className="desktop-update-banner" role="status">
      <div className="desktop-update-copy">
        <strong>{desktopUpdateHeadline(update.prompt, update.pendingRelaunch)}</strong>
        <p>{desktopUpdateBody(update.prompt, update.pendingRelaunch, update.nativeInstall)}</p>
      </div>
      <div className="desktop-update-actions">
        {update.pendingRelaunch ? (
          <button className="secondary-command" onClick={update.dismiss} type="button">知道了</button>
        ) : (
          <>
            {update.required ? null : (
              <button className="secondary-command" disabled={update.installing} onClick={update.dismiss} type="button">稍后</button>
            )}
            <SaveInstallerLink className="primary-action" onDownloaded={update.check} release={update.prompt}>
              <Download size={16} />
              下载 {update.prompt.versionName}
            </SaveInstallerLink>
          </>
        )}
      </div>
      {update.error ? <span className="form-error" role="alert">{update.error}</span> : null}
    </div>
  )
}

export function DesktopUpdateSettings({ update }: { update: DesktopUpdateState }) {
  if (!update.available) return null
  const label = update.pendingRelaunch
    ? `已下载 ${update.release?.versionName}，请完全退出后再从「应用程序」打开`
    : update.release
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
        {update.release && !update.pendingRelaunch ? (
          <SaveInstallerLink className="secondary-command" onDownloaded={update.check} release={update.release}>
            <Download size={16} />
            下载 {update.release.versionName}
          </SaveInstallerLink>
        ) : null}
      </div>
      {update.error && update.release ? <span className="form-error" role="alert">{update.error}</span> : null}
    </section>
  )
}

function SaveInstallerLink({
  children,
  className,
  onDownloaded,
  release,
}: {
  children: ReactNode
  className: string
  onDownloaded?: () => void
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
            onDownloaded?.()
            setStatus(`已保存 ${fileName}`)
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

