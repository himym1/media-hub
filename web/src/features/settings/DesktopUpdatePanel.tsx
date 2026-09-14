import { Download, RefreshCw } from 'lucide-react'
import { desktopPlatformFromPath, formatDesktopUpdateSize } from '../../shared/desktop/desktopUpdate'
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
        <button className="primary-action" disabled={update.installing} onClick={() => void update.install()} type="button">
          <Download size={16} />
          {update.installing ? '正在下载并安装…' : `${update.nativeInstall ? '安装' : '下载'} ${update.prompt.versionName}`}
        </button>
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
      <button
        className="secondary-command"
        disabled={update.checking || update.installing}
        onClick={() => { if (update.release) void update.install(); else void update.check() }}
        type="button"
      >
        {update.release ? <Download size={16} /> : <RefreshCw size={16} />}
        {update.installing ? '正在下载并安装…' : update.release ? `${update.nativeInstall ? '安装' : '下载'} ${update.release.versionName}` : update.checking ? '正在检查' : '检查更新'}
      </button>
      {update.error && update.release ? <span className="form-error" role="alert">{update.error}</span> : null}
    </section>
  )
}

function updateHint(nativeInstall: boolean, downloadPath: string) {
  if (desktopPlatformFromPath(downloadPath) === 'darwin') {
    return nativeInstall ? '下载后会打开安装盘，请把 Media Hub 拖到「应用程序」后重新打开' : '下载后请退出应用，打开安装盘并把 Media Hub 拖到「应用程序」'
  }
  return nativeInstall ? '下载后会关闭应用并安装' : '下载后请退出应用再运行安装包'
}
