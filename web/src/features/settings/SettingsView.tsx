import { useCallback, useEffect, useRef, useState, type KeyboardEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Activity, CalendarCheck, Database, FileVideo, Film, HardDrive, KeyRound, LogOut, QrCode, RefreshCw, Server, SlidersHorizontal } from 'lucide-react'
import {
  changePassword,
  getDrive115Status,
  getEmbyLibraries,
  getOperationalStatistics,
  getProviderSettings,
  getSTRMStatus,
  getSourceCheckIns,
  pollDrive115Authorization,
  retrySourceCheckIn,
  startDrive115Authorization,
  syncSTRMLibrary,
  testWeComNotification,
  updateProviderSettings,
  type Integration,
  type IntegrationStatus,
} from '../../shared/api/mediaHub'
import type { DesktopUpdateState } from '../../shared/desktop/useDesktopUpdate'
import { commitUrl } from '../../shared/navigation/urlState'
import { IconButton } from '../../shared/ui/IconButton'
import { DesktopUpdateSettings } from './DesktopUpdatePanel'
import { ProviderSettingsForm } from './ProviderSettingsForm'

const statusLabel: Record<IntegrationStatus, string> = { healthy: '在线', degraded: '受限', unavailable: '离线', unconfigured: '未配置' }
const checkInStateLabel = (state?: string) => {
  switch (state) {
    case 'completed': return '已签到'
    case 'skipped': return '已跳过'
    case 'failed': return '失败'
    case 'needs_attention': return '需要确认'
    case 'running': return '进行中'
    default: return '等待执行'
  }
}
type SettingsSection = 'overview' | 'providers' | 'account'
const settingsSections: SettingsSection[] = ['overview', 'providers', 'account']

function sectionFromLocation(): SettingsSection {
  const value = new URLSearchParams(window.location.search).get('settings')
  return settingsSections.includes(value as SettingsSection) ? value as SettingsSection : 'overview'
}

function formatCapacity(bytes?: number) {
  if (bytes === undefined) return '未知'
  return `${(bytes / 1024 / 1024 / 1024 / 1024).toFixed(2)} TB`
}

type SettingsViewProps = {
  desktopUpdate: DesktopUpdateState
  integrations: Integration[]
  onDirtyChange: (dirty: boolean) => void
  onLogout: () => void
  onRefresh: () => void
}

export function SettingsView({ desktopUpdate, integrations, onDirtyChange, onLogout, onRefresh }: SettingsViewProps) {
  const [section, setSection] = useState<SettingsSection>(sectionFromLocation)
  const [providerDirty, setProviderDirty] = useState(false)
  const providerDirtyRef = useRef(false)
  const providerDirtyUrl = useRef<string | null>(null)
  const updateProviderDirty = useCallback((dirty: boolean) => {
    if (dirty && providerDirtyUrl.current === null) providerDirtyUrl.current = window.location.href
    if (!dirty) providerDirtyUrl.current = null
    providerDirtyRef.current = dirty
    setProviderDirty(dirty)
    onDirtyChange(dirty)
  }, [onDirtyChange])
  const strm = useQuery({ queryKey: ['strm-status'], queryFn: getSTRMStatus, enabled: section === 'overview', retry: false, refetchInterval: (state) => state.state.data?.running ? 3_000 : false })
  const drive = useQuery({ queryKey: ['drive-115-status'], queryFn: getDrive115Status, enabled: section === 'overview', retry: false })
  const refetchDrive = drive.refetch
  const emby = useQuery({ queryKey: ['emby-libraries'], queryFn: getEmbyLibraries, enabled: section === 'overview', retry: false })
  const checkins = useQuery({ queryKey: ['source-checkins'], queryFn: getSourceCheckIns, enabled: section === 'overview', retry: false, refetchInterval: 15_000 })
  const queryClient = useQueryClient()
  const providerSettings = useQuery({ queryKey: ['provider-settings'], queryFn: getProviderSettings, enabled: section === 'providers' })
  const saveProviderSettings = useMutation({
    mutationFn: updateProviderSettings,
    onSuccess: async (value) => {
      queryClient.setQueryData(['provider-settings'], value)
      updateProviderDirty(false)
      await Promise.all([strm.refetch(), drive.refetch(), emby.refetch(), checkins.refetch(), queryClient.invalidateQueries({ queryKey: ['system-overview'] })])
      onRefresh()
    },
  })
  const syncSTRM = useMutation({
    mutationFn: () => syncSTRMLibrary({ mediaType: 'all' }),
    onSuccess: async () => {
      await Promise.all([strm.refetch(), queryClient.invalidateQueries({ queryKey: ['system-overview'] })])
      onRefresh()
    },
  })
  const testWeCom = useMutation({ mutationFn: testWeComNotification })
  const retryCheckIn = useMutation({
    mutationFn: retrySourceCheckIn,
    onSuccess: async () => {
      await Promise.all([checkins.refetch(), queryClient.invalidateQueries({ queryKey: ['system-overview'] })])
      onRefresh()
    },
  })
  const statistics = useQuery({ queryKey: ['operational-statistics'], queryFn: getOperationalStatistics, enabled: section === 'overview' })
  const authorization = useMutation({ mutationFn: startDrive115Authorization })
  const authorizationStatus = useQuery({
    queryKey: ['drive-115-authorization', authorization.data?.id],
    queryFn: () => pollDrive115Authorization(authorization.data!.id),
    enabled: Boolean(authorization.data?.id),
    refetchInterval: (state) => state.state.data?.state === 'confirmed' || state.state.data?.state === 'expired' ? false : 2_000,
    retry: false,
  })
  const authorizationState = authorizationStatus.data?.state
  useEffect(() => { if (authorizationState === 'confirmed') void refetchDrive() }, [authorizationState, refetchDrive])

  useEffect(() => {
    const preventUnload = (event: BeforeUnloadEvent) => {
      if (!providerDirtyRef.current) return
      event.preventDefault()
    }
    window.addEventListener('beforeunload', preventUnload)
    return () => window.removeEventListener('beforeunload', preventUnload)
  }, [])

  useEffect(() => {
    const restoreSection = () => {
      if (providerDirtyRef.current && providerDirtyUrl.current && window.location.href !== providerDirtyUrl.current) {
        if (!window.confirm('Provider 设置尚未保存。确定离开并放弃修改吗？')) {
          window.history.pushState({}, '', providerDirtyUrl.current)
          return
        }
      }
      updateProviderDirty(false)
      setSection(sectionFromLocation())
    }
    window.addEventListener('popstate', restoreSection)
    return () => window.removeEventListener('popstate', restoreSection)
  }, [updateProviderDirty])

  const changeSection = (next: SettingsSection) => {
    if (next === section) return
    if (section === 'providers' && providerDirty && !window.confirm('Provider 设置尚未保存。确定离开并放弃修改吗？')) return
    updateProviderDirty(false)
    setSection(next)
    commitUrl({ settings: next })
  }

  const handleTabKey = (event: KeyboardEvent<HTMLButtonElement>, current: SettingsSection) => {
    if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return
    event.preventDefault()
    const direction = event.key === 'ArrowRight' ? 1 : -1
    const index = settingsSections.indexOf(current)
    const next = settingsSections[(index + direction + settingsSections.length) % settingsSections.length]
    changeSection(next)
    requestAnimationFrame(() => document.getElementById(`settings-tab-${next}`)?.focus())
  }

  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmation, setConfirmation] = useState('')
  const password = useMutation({
    mutationFn: () => changePassword(currentPassword, newPassword),
    onSuccess: () => { setCurrentPassword(''); setNewPassword(''); setConfirmation('') },
  })
  const refreshAll = () => {
    onRefresh()
    void strm.refetch()
    void drive.refetch()
    void emby.refetch()
    void statistics.refetch()
    void providerSettings.refetch()
    void checkins.refetch()
    void desktopUpdate.check()
  }

  const checkUpdate = desktopUpdate.check
  useEffect(() => {
    if (section !== 'account') return
    void checkUpdate()
  }, [checkUpdate, section])

  return (
    <section className="workspace-view settings-view">
      <header className="view-header compact-view-header">
        <div><h1>系统设置</h1><p>运行状态、服务接入和账户安全分开管理。</p></div>
        <IconButton label="刷新所有服务" onClick={refreshAll} subtle><RefreshCw size={17} /></IconButton>
      </header>
      <div className="settings-tabs" role="tablist" aria-label="设置分类">
        <button aria-controls="settings-panel-overview" aria-selected={section === 'overview'} id="settings-tab-overview" onClick={() => changeSection('overview')} onKeyDown={(event) => handleTabKey(event, 'overview')} role="tab" tabIndex={section === 'overview' ? 0 : -1} type="button"><Activity size={16} />概览</button>
        <button aria-controls="settings-panel-providers" aria-selected={section === 'providers'} id="settings-tab-providers" onClick={() => changeSection('providers')} onKeyDown={(event) => handleTabKey(event, 'providers')} role="tab" tabIndex={section === 'providers' ? 0 : -1} type="button"><SlidersHorizontal size={16} />服务接入{providerDirty ? <span className="unsaved-indicator">未保存</span> : null}</button>
        <button aria-controls="settings-panel-account" aria-selected={section === 'account'} id="settings-tab-account" onClick={() => changeSection('account')} onKeyDown={(event) => handleTabKey(event, 'account')} role="tab" tabIndex={section === 'account' ? 0 : -1} type="button"><KeyRound size={16} />账户</button>
      </div>

      {section === 'overview' ? <div aria-labelledby="settings-tab-overview" className="settings-section" id="settings-panel-overview" role="tabpanel">
        {statistics.data ? <section className="statistics-strip" aria-label="运营摘要"><div><Activity size={18} /><span>运营摘要</span></div><dl><div><dt>进行中任务</dt><dd>{statistics.data.transfersActive}</dd></div><div><dt>需要处理</dt><dd>{statistics.data.transfersNeedsAttention + statistics.data.commandsNeedsAttention + statistics.data.notificationsNeedsAttention}</dd></div><div><dt>启用订阅</dt><dd>{statistics.data.subscriptionsEnabled}</dd></div><div><dt>失败运行</dt><dd>{statistics.data.runsFailed}</dd></div></dl></section> : null}
        <div className="service-grid">{integrations.filter((integration) => integration.id !== 'qmediasync').map((integration) => { const Icon = integration.id === '115' ? HardDrive : integration.id === 'strm' ? FileVideo : integration.id === 'emby' ? Film : integration.id === 'source-checkin' ? CalendarCheck : Server; return <article className={`service-card ${integration.status}`} key={integration.id}><Icon size={20} /><div><strong>{integration.label}</strong><span>{integration.detail}</span></div><span className={`state-chip ${integration.status}`}>{statusLabel[integration.status]}</span></article> })}</div>
        <div className="diagnostic-grid">
          <section className="diagnostic-block"><div className="diagnostic-title"><FileVideo size={18} /><strong>内置 STRM</strong></div>
            <dl>
              <div><dt>模式</dt><dd>内置写入</dd></div>
              <div><dt>挂载</dt><dd>{strm.data?.mountWritable ? '可写' : (strm.data?.mountPath ? '不可写' : '未配置')}</dd></div>
              <div><dt>115 会话</dt><dd>{strm.data?.sessionOk ? '有效' : '不可用'}</dd></div>
              <div><dt>最近同步</dt><dd>{strm.data?.running ? '进行中' : (strm.data?.lastSummary || '尚无记录')}</dd></div>
              {strm.data?.lastError ? <div><dt>失败原因</dt><dd>{strm.data.lastError}</dd></div> : null}
            </dl>
            <button className="secondary-command" disabled={syncSTRM.isPending || strm.data?.running} onClick={() => syncSTRM.mutate()} type="button">{syncSTRM.isPending || strm.data?.running ? '正在同步…' : '立即同步'}</button>
            {syncSTRM.isError ? <span className="form-error" role="alert">{syncSTRM.error.message}</span> : null}
          </section>
          <section className="diagnostic-block drive-authorization"><div className="diagnostic-title"><HardDrive size={18} /><strong>115</strong></div><dl><div><dt>授权</dt><dd>{drive.data?.authorized ? '有效' : '不可用'}</dd></div><div><dt>已使用</dt><dd>{formatCapacity(drive.data?.usedBytes)}</dd></div><div><dt>总容量</dt><dd>{formatCapacity(drive.data?.totalBytes)}</dd></div></dl>{authorization.data?.qrImage && authorizationStatus.data?.state !== 'confirmed' ? <div className="qr-box"><img alt="115 扫码授权二维码" height="148" src={authorization.data.qrImage} width="148" /></div> : null}<button className="secondary-command" disabled={authorization.isPending || authorizationStatus.data?.state === 'pending'} onClick={() => authorization.mutate()} type="button"><QrCode size={15} />{authorizationStatus.data?.state === 'confirmed' ? '重新授权' : authorizationStatus.data?.state === 'pending' ? '等待扫码确认' : '扫码授权'}</button>{authorization.isError ? <span className="form-error" role="alert">{authorization.error.message}</span> : null}</section>
          <section className="diagnostic-block"><div className="diagnostic-title"><Database size={18} /><strong>Emby</strong></div><dl><div><dt>媒体库</dt><dd>{emby.data?.libraries.length ?? 0}</dd></div><div><dt>读取状态</dt><dd>{emby.isSuccess ? '正常' : '不可用'}</dd></div></dl></section>
          <section className="diagnostic-block source-checkins"><div className="diagnostic-title"><CalendarCheck size={18} /><strong>资源签到</strong></div>
            {checkins.data?.items.length ? <ul className="checkin-list">{checkins.data.items.map((item) => (
              <li key={item.sourceId}>
                <div><strong>{item.label}</strong><span>{checkInStateLabel(item.state)}{item.message ? ` · ${item.message}` : ''}</span></div>
                <button className="secondary-command" disabled={item.state === 'running' || retryCheckIn.isPending} onClick={() => retryCheckIn.mutate(item.sourceId)} type="button">立即签到</button>
              </li>
            ))}</ul> : <p className="empty-inline">{checkins.isLoading ? '正在读取签到状态…' : '尚未配置可签到的资源源'}</p>}
            {retryCheckIn.isError ? <span className="form-error" role="alert">{retryCheckIn.error.message}</span> : null}
          </section>
        </div>
      </div> : null}

      {section === 'providers' ? <div aria-labelledby="settings-tab-providers" className="settings-section" id="settings-panel-providers" role="tabpanel">{providerSettings.data ? <ProviderSettingsForm error={saveProviderSettings.error?.message} isSaving={saveProviderSettings.isPending} saved={saveProviderSettings.isSuccess} isTesting={testWeCom.isPending} testError={testWeCom.error?.message} tested={testWeCom.isSuccess} onDirty={() => { updateProviderDirty(true); saveProviderSettings.reset(); testWeCom.reset() }} onSave={(input) => saveProviderSettings.mutate(input)} onTest={() => testWeCom.mutate()} settings={providerSettings.data} /> : providerSettings.isLoading ? <p className="empty-inline">正在读取 Provider 设置…</p> : <p className="form-error" role="alert">{providerSettings.error?.message ?? '无法读取 Provider 设置'}</p>}</div> : null}

      {section === 'account' ? <div aria-labelledby="settings-tab-account" className="settings-section account-settings" id="settings-panel-account" role="tabpanel">
        <form className="password-form password-form-prominent" onSubmit={(event) => { event.preventDefault(); password.mutate() }}>
          <div className="diagnostic-title"><KeyRound size={18} /><strong>管理员密码</strong><span>修改密码后将撤销其他设备会话</span></div>
          <label><span>当前密码</span><input autoComplete="current-password" maxLength={1024} name="current-password" onChange={(event) => setCurrentPassword(event.target.value)} required type="password" value={currentPassword} /></label>
          <label><span>新密码</span><input autoComplete="new-password" minLength={12} maxLength={1024} name="new-password" onChange={(event) => setNewPassword(event.target.value)} required type="password" value={newPassword} /></label>
          <label><span>确认新密码</span><input autoComplete="new-password" minLength={12} maxLength={1024} name="confirm-password" onChange={(event) => setConfirmation(event.target.value)} required type="password" value={confirmation} /></label>
          {password.isError ? <span className="form-error" role="alert">{password.error.message}</span> : null}
          {password.isSuccess ? <span className="form-success" role="status">密码已修改，其他设备的会话已撤销</span> : null}
          <button className="primary-action" disabled={password.isPending || newPassword.length < 12 || newPassword !== confirmation || currentPassword === newPassword} type="submit"><KeyRound size={16} />{password.isPending ? '正在修改…' : '修改密码'}</button>
        </form>
        <DesktopUpdateSettings update={desktopUpdate} />
        <section className="session-actions">
          <div><strong>当前会话</strong><span>退出后需要重新输入管理员密码。</span></div>
          <button className="secondary-command" onClick={onLogout} type="button"><LogOut size={16} />退出登录</button>
        </section>
      </div> : null}
    </section>
  )
}
