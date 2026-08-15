import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Activity, Database, Film, HardDrive, KeyRound, LogOut, QrCode, RefreshCw, Server, SlidersHorizontal, Waypoints } from 'lucide-react'
import {
  changePassword,
  getDrive115Status,
  getEmbyLibraries,
  getOperationalStatistics,
  getProviderSettings,
  getQMediaSyncStatus,
  pollDrive115Authorization,
  startDrive115Authorization,
  testWeComNotification,
  updateProviderSettings,
  type Integration,
  type IntegrationStatus,
} from '../../shared/api/mediaHub'
import { IconButton } from '../../shared/ui/IconButton'
import { ProviderSettingsForm } from './ProviderSettingsForm'

const statusLabel: Record<IntegrationStatus, string> = { healthy: '在线', degraded: '受限', unavailable: '离线', unconfigured: '未配置' }
type SettingsSection = 'overview' | 'providers' | 'account'

function formatCapacity(bytes?: number) {
  if (bytes === undefined) return '未知'
  return `${(bytes / 1024 / 1024 / 1024 / 1024).toFixed(2)} TB`
}

type SettingsViewProps = {
  integrations: Integration[]
  onLogout: () => void
  onRefresh: () => void
}

export function SettingsView({ integrations, onLogout, onRefresh }: SettingsViewProps) {
  const [section, setSection] = useState<SettingsSection>('overview')
  const qms = useQuery({ queryKey: ['qms-status'], queryFn: getQMediaSyncStatus, retry: false })
  const drive = useQuery({ queryKey: ['drive-115-status'], queryFn: getDrive115Status, retry: false })
  const refetchDrive = drive.refetch
  const emby = useQuery({ queryKey: ['emby-libraries'], queryFn: getEmbyLibraries, retry: false })
  const queryClient = useQueryClient()
  const providerSettings = useQuery({ queryKey: ['provider-settings'], queryFn: getProviderSettings })
  const saveProviderSettings = useMutation({
    mutationFn: updateProviderSettings,
    onSuccess: async (value) => {
      queryClient.setQueryData(['provider-settings'], value)
      await Promise.all([qms.refetch(), drive.refetch(), emby.refetch(), queryClient.invalidateQueries({ queryKey: ['system-overview'] })])
      onRefresh()
    },
  })
  const testWeCom = useMutation({ mutationFn: testWeComNotification })
  const statistics = useQuery({ queryKey: ['operational-statistics'], queryFn: getOperationalStatistics })
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

  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmation, setConfirmation] = useState('')
  const password = useMutation({
    mutationFn: () => changePassword(currentPassword, newPassword),
    onSuccess: () => { setCurrentPassword(''); setNewPassword(''); setConfirmation('') },
  })
  const refreshAll = () => {
    onRefresh()
    void qms.refetch()
    void drive.refetch()
    void emby.refetch()
    void statistics.refetch()
    void providerSettings.refetch()
  }

  return (
    <section className="workspace-view settings-view">
      <header className="view-header compact-view-header">
        <div><p className="eyebrow">SYSTEM</p><h1>服务与设置</h1><p>运行状态、Provider 配置和账户安全分开管理。</p></div>
        <IconButton label="刷新所有服务" onClick={refreshAll} subtle><RefreshCw size={17} /></IconButton>
      </header>
      <div className="settings-tabs" role="tablist" aria-label="设置分类">
        <button aria-selected={section === 'overview'} onClick={() => setSection('overview')} role="tab" type="button"><Activity size={16} />概览</button>
        <button aria-selected={section === 'providers'} onClick={() => setSection('providers')} role="tab" type="button"><SlidersHorizontal size={16} />Provider</button>
        <button aria-selected={section === 'account'} onClick={() => setSection('account')} role="tab" type="button"><KeyRound size={16} />账户</button>
      </div>

      {section === 'overview' ? <div className="settings-section" role="tabpanel">
        {statistics.data ? <section className="statistics-strip" aria-label="运营摘要"><div><Activity size={18} /><span>运营摘要</span></div><dl><div><dt>进行中任务</dt><dd>{statistics.data.transfersActive}</dd></div><div><dt>需要处理</dt><dd>{statistics.data.transfersNeedsAttention + statistics.data.commandsNeedsAttention + statistics.data.notificationsNeedsAttention}</dd></div><div><dt>启用订阅</dt><dd>{statistics.data.subscriptionsEnabled}</dd></div><div><dt>失败运行</dt><dd>{statistics.data.runsFailed}</dd></div></dl></section> : null}
        <div className="service-grid">{integrations.map((integration) => { const Icon = integration.id === '115' ? HardDrive : integration.id === 'qmediasync' ? Waypoints : integration.id === 'emby' ? Film : Server; return <article className="service-card" key={integration.id}><Icon size={20} /><div><strong>{integration.label}</strong><span>{integration.detail}</span></div><span className={`state-chip ${integration.status}`}>{statusLabel[integration.status]}</span></article> })}</div>
        <div className="diagnostic-grid">
          <section className="diagnostic-block"><div className="diagnostic-title"><Waypoints size={18} /><strong>QMediaSync</strong></div><dl><div><dt>版本</dt><dd>{qms.data?.version ?? '不可用'}</dd></div><div><dt>同步记录</dt><dd>{qms.data?.totalSyncs ?? 0}</dd></div><div><dt>最近状态</dt><dd>{qms.data?.recentSyncs[0]?.state ?? '无记录'}</dd></div></dl></section>
          <section className="diagnostic-block drive-authorization"><div className="diagnostic-title"><HardDrive size={18} /><strong>115</strong></div><dl><div><dt>授权</dt><dd>{drive.data?.authorized ? '有效' : '不可用'}</dd></div><div><dt>已使用</dt><dd>{formatCapacity(drive.data?.usedBytes)}</dd></div><div><dt>总容量</dt><dd>{formatCapacity(drive.data?.totalBytes)}</dd></div></dl>{authorization.data?.qrImage && authorizationStatus.data?.state !== 'confirmed' ? <img alt="115 扫码授权二维码" height="148" src={authorization.data.qrImage} width="148" /> : null}<button className="secondary-command" disabled={authorization.isPending || authorizationStatus.data?.state === 'pending'} onClick={() => authorization.mutate()} type="button"><QrCode size={15} />{authorizationStatus.data?.state === 'confirmed' ? '重新授权' : authorizationStatus.data?.state === 'pending' ? '等待扫码确认' : '扫码授权'}</button>{authorization.isError ? <span className="form-error" role="alert">{authorization.error.message}</span> : null}</section>
          <section className="diagnostic-block"><div className="diagnostic-title"><Database size={18} /><strong>Emby</strong></div><dl><div><dt>媒体库</dt><dd>{emby.data?.libraries.length ?? 0}</dd></div><div><dt>读取状态</dt><dd>{emby.isSuccess ? '正常' : '不可用'}</dd></div></dl></section>
        </div>
      </div> : null}

      {section === 'providers' ? <div className="settings-section" role="tabpanel">{providerSettings.data ? <ProviderSettingsForm error={saveProviderSettings.error?.message} isSaving={saveProviderSettings.isPending} saved={saveProviderSettings.isSuccess} isTesting={testWeCom.isPending} testError={testWeCom.error?.message} tested={testWeCom.isSuccess} onDirty={() => { saveProviderSettings.reset(); testWeCom.reset() }} onSave={(input) => saveProviderSettings.mutate(input)} onTest={() => testWeCom.mutate()} settings={providerSettings.data} /> : providerSettings.isLoading ? <p className="empty-inline">正在读取 Provider 设置…</p> : <p className="form-error" role="alert">{providerSettings.error?.message ?? '无法读取 Provider 设置'}</p>}</div> : null}

      {section === 'account' ? <div className="settings-section account-settings" role="tabpanel">
        <form className="password-form password-form-prominent" onSubmit={(event) => { event.preventDefault(); password.mutate() }}>
          <div className="diagnostic-title"><KeyRound size={18} /><strong>管理员密码</strong><span>修改密码后将撤销其他设备会话</span></div>
          <label><span>当前密码</span><input autoComplete="current-password" maxLength={1024} name="current-password" onChange={(event) => setCurrentPassword(event.target.value)} required type="password" value={currentPassword} /></label>
          <label><span>新密码</span><input autoComplete="new-password" minLength={12} maxLength={1024} name="new-password" onChange={(event) => setNewPassword(event.target.value)} required type="password" value={newPassword} /></label>
          <label><span>确认新密码</span><input autoComplete="new-password" minLength={12} maxLength={1024} name="confirm-password" onChange={(event) => setConfirmation(event.target.value)} required type="password" value={confirmation} /></label>
          {password.isError ? <span className="form-error" role="alert">{password.error.message}</span> : null}
          {password.isSuccess ? <span className="form-success" role="status">密码已修改，其他设备的会话已撤销</span> : null}
          <button className="primary-action" disabled={password.isPending || newPassword.length < 12 || newPassword !== confirmation || currentPassword === newPassword} type="submit"><KeyRound size={16} />{password.isPending ? '正在修改…' : '修改密码'}</button>
        </form>
        <section className="session-actions">
          <div><strong>当前会话</strong><span>退出后需要重新输入管理员密码。</span></div>
          <button className="secondary-command" onClick={onLogout} type="button"><LogOut size={16} />退出登录</button>
        </section>
      </div> : null}
    </section>
  )
}
