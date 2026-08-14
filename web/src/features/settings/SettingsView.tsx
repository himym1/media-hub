import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Activity, Database, Film, HardDrive, KeyRound, QrCode, RefreshCw, Server, Waypoints } from 'lucide-react'
import {
  changePassword,
  getDrive115Status,
  getEmbyLibraries,
  getProviderSettings,
  getQMediaSyncStatus,
  getOperationalStatistics,
  pollDrive115Authorization,
  startDrive115Authorization,
  testWeComNotification,
  updateProviderSettings,
  type Integration,
  type IntegrationStatus,
} from '../../shared/api/mediaHub'
import { ProviderSettingsForm } from './ProviderSettingsForm'
import { IconButton } from '../../shared/ui/IconButton'

const statusLabel: Record<IntegrationStatus, string> = {
  healthy: '在线',
  degraded: '受限',
  unavailable: '离线',
  unconfigured: '未配置',
}

function formatCapacity(bytes?: number) {
  if (bytes === undefined) return '未知'
  const tebibytes = bytes / 1024 / 1024 / 1024 / 1024
  return `${tebibytes.toFixed(2)} TB`
}

type SettingsViewProps = {
  integrations: Integration[]
  onRefresh: () => void
}

export function SettingsView({ integrations, onRefresh }: SettingsViewProps) {
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
  useEffect(() => {
    if (authorizationState === 'confirmed') void refetchDrive()
  }, [authorizationState, refetchDrive])
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmation, setConfirmation] = useState('')
  const password = useMutation({
    mutationFn: () => changePassword(currentPassword, newPassword),
    onSuccess: () => {
      setCurrentPassword('')
      setNewPassword('')
      setConfirmation('')
    },
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
    <section className="workspace-view">
      <div className="workspace-heading">
        <div><p className="eyebrow">INTEGRATIONS</p><h1>服务连接</h1></div>
        <div className="workspace-heading-actions">
          <IconButton label="修改管理员密码" onClick={() => { document.querySelector<HTMLInputElement>('#current-admin-password')?.focus(); document.querySelector('#admin-password-form')?.scrollIntoView({ behavior: 'smooth', block: 'center' }) }} subtle><KeyRound size={17} /></IconButton>
          <IconButton label="刷新所有服务" onClick={refreshAll} subtle><RefreshCw size={17} /></IconButton>
        </div>
      </div>

      <div className="service-grid">
        {integrations.map((integration) => {
          const Icon = integration.id === '115' ? HardDrive : integration.id === 'qmediasync' ? Waypoints : integration.id === 'emby' ? Film : Server
          return <article className="service-card" key={integration.id}><Icon size={20} /><div><strong>{integration.label}</strong><span>{integration.detail}</span></div><span className={`state-chip ${integration.status}`}>{statusLabel[integration.status]}</span></article>
        })}
      </div>

      <form className="password-form password-form-prominent" id="admin-password-form" onSubmit={(event) => { event.preventDefault(); password.mutate() }}>
        <div className="diagnostic-title"><KeyRound size={18} /><strong>管理员密码</strong><span>修改当前账户密码，并撤销其他设备会话</span></div>
        <label><span>当前密码</span><input autoComplete="current-password" id="current-admin-password" maxLength={1024} onChange={(event) => setCurrentPassword(event.target.value)} required type="password" value={currentPassword} /></label>
        <label><span>新密码</span><input autoComplete="new-password" minLength={12} maxLength={1024} onChange={(event) => setNewPassword(event.target.value)} required type="password" value={newPassword} /></label>
        <label><span>确认新密码</span><input autoComplete="new-password" minLength={12} maxLength={1024} onChange={(event) => setConfirmation(event.target.value)} required type="password" value={confirmation} /></label>
        {password.isError ? <span className="form-error" role="alert">{password.error.message}</span> : null}
        {password.isSuccess ? <span className="form-success" role="status">密码已修改，其他设备的会话已撤销</span> : null}
        <button className="primary-action" disabled={password.isPending || newPassword.length < 12 || newPassword !== confirmation || currentPassword === newPassword} type="submit"><KeyRound size={16} />{password.isPending ? '正在修改' : '修改密码'}</button>
      </form>

      {providerSettings.data ? <ProviderSettingsForm
        error={saveProviderSettings.error?.message} isSaving={saveProviderSettings.isPending} saved={saveProviderSettings.isSuccess}
        isTesting={testWeCom.isPending} testError={testWeCom.error?.message} tested={testWeCom.isSuccess}
        onDirty={() => { saveProviderSettings.reset(); testWeCom.reset() }} onSave={(input) => saveProviderSettings.mutate(input)} onTest={() => testWeCom.mutate()}
        settings={providerSettings.data}
      /> : providerSettings.isLoading ? <p className="empty-inline">正在读取服务设置</p> : <p className="form-error" role="alert">{providerSettings.error?.message ?? '无法读取服务设置'}</p>}

      <div className="diagnostic-grid">
        <section className="diagnostic-block"><div className="diagnostic-title"><Waypoints size={18} /><strong>QMediaSync</strong></div><dl><div><dt>版本</dt><dd>{qms.data?.version ?? '不可用'}</dd></div><div><dt>同步记录</dt><dd>{qms.data?.totalSyncs ?? 0}</dd></div><div><dt>最近状态</dt><dd>{qms.data?.recentSyncs[0]?.state ?? '无记录'}</dd></div></dl></section>
        <section className="diagnostic-block drive-authorization"><div className="diagnostic-title"><HardDrive size={18} /><strong>115</strong></div><dl><div><dt>授权</dt><dd>{drive.data?.authorized ? '有效' : '不可用'}</dd></div><div><dt>已使用</dt><dd>{formatCapacity(drive.data?.usedBytes)}</dd></div><div><dt>总容量</dt><dd>{formatCapacity(drive.data?.totalBytes)}</dd></div></dl>{authorization.data?.qrImage && authorizationStatus.data?.state !== 'confirmed' ? <img alt="115 扫码授权二维码" src={authorization.data.qrImage} /> : null}<button className="secondary-command" disabled={authorization.isPending || authorizationStatus.data?.state === 'pending'} onClick={() => authorization.mutate()} type="button"><QrCode size={15} />{authorizationStatus.data?.state === 'confirmed' ? '重新授权' : authorizationStatus.data?.state === 'pending' ? '等待扫码确认' : '扫码授权'}</button>{authorization.isError ? <span className="form-error" role="alert">{authorization.error.message}</span> : null}</section>
        <section className="diagnostic-block"><div className="diagnostic-title"><Database size={18} /><strong>Emby</strong></div><dl><div><dt>媒体库</dt><dd>{emby.data?.libraries.length ?? 0}</dd></div><div><dt>读取状态</dt><dd>{emby.isSuccess ? '正常' : '不可用'}</dd></div></dl></section>
      </div>

      {statistics.data ? <section className="statistics-strip" aria-label="运营摘要"><div><Activity size={18} /><span>运营摘要</span></div><dl><div><dt>进行中任务</dt><dd>{statistics.data.transfersActive}</dd></div><div><dt>需要处理</dt><dd>{statistics.data.transfersNeedsAttention + statistics.data.commandsNeedsAttention + statistics.data.notificationsNeedsAttention}</dd></div><div><dt>启用订阅</dt><dd>{statistics.data.subscriptionsEnabled}</dd></div><div><dt>失败运行</dt><dd>{statistics.data.runsFailed}</dd></div></dl></section> : null}

    </section>
  )
}
