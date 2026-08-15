import { useCallback, useEffect, useRef, useState, type MouseEvent } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Film, LayoutGrid, LibraryBig, ListPlus, ListTodo, LogOut, Settings2, TerminalSquare } from 'lucide-react'
import { getSystemOverview, listTransfers, type Candidate } from '../../shared/api/mediaHub'
import { commitUrl } from '../../shared/navigation/urlState'
import { LibraryView } from '../library/LibraryView'
import { OperationsView } from '../operations/OperationsView'
import { SettingsView } from '../settings/SettingsView'
import { SubscriptionView } from '../subscriptions/SubscriptionView'
import { TransferQueue } from '../transfers/TransferQueue'
import { DiscoveryView } from './DiscoveryView'
import './SearchWorkspace.css'

type WorkspaceView = '发现' | '任务' | '订阅' | '媒体库' | '运维' | '服务'

const mediaNavItems = [
  { label: '发现', icon: LayoutGrid },
  { label: '任务', icon: ListTodo },
  { label: '订阅', icon: ListPlus },
  { label: '媒体库', icon: LibraryBig },
] satisfies { label: WorkspaceView; icon: typeof LayoutGrid }[]

const systemNavItems = [
  { label: '运维', icon: TerminalSquare },
  { label: '服务', icon: Settings2 },
] satisfies { label: WorkspaceView; icon: typeof LayoutGrid }[]

const viewSlugs: Record<WorkspaceView, string> = {
  发现: 'discover',
  任务: 'transfers',
  订阅: 'subscriptions',
  媒体库: 'library',
  运维: 'operations',
  服务: 'settings',
}
const slugViews = Object.fromEntries(Object.entries(viewSlugs).map(([view, slug]) => [slug, view])) as Record<string, WorkspaceView>
const activeTransferStates = new Set(['queued', 'transferring', 'transferred', 'submitting_sync', 'syncing', 'retry_wait', 'refreshing_emby', 'indexing_emby', 'verifying_playback'])

type SearchWorkspaceProps = {
  isLoggingOut: boolean
  onLogout: () => void
}

function viewFromLocation(): WorkspaceView {
  return slugViews[new URLSearchParams(window.location.search).get('view') ?? ''] ?? '发现'
}

export function SearchWorkspace({ isLoggingOut, onLogout }: SearchWorkspaceProps) {
  const [activeView, setActiveView] = useState<WorkspaceView>(viewFromLocation)
  const [draftSubscription, setDraftSubscription] = useState<Candidate | null>(null)
  const [providerSettingsDirty, setProviderSettingsDirty] = useState(false)
  const providerDirtyUrl = useRef<string | null>(null)
  const updateProviderDirty = useCallback((dirty: boolean) => {
    if (dirty && providerDirtyUrl.current === null) providerDirtyUrl.current = window.location.href
    if (!dirty) providerDirtyUrl.current = null
    setProviderSettingsDirty(dirty)
  }, [])

  useEffect(() => {
    const onPopState = () => {
      if (providerSettingsDirty && providerDirtyUrl.current && window.location.href !== providerDirtyUrl.current) {
        if (!window.confirm('Provider 设置尚未保存。确定离开并放弃修改吗？')) {
          window.history.pushState({}, '', providerDirtyUrl.current)
          return
        }
        updateProviderDirty(false)
      }
      setActiveView(viewFromLocation())
    }
    window.addEventListener('popstate', onPopState, { capture: true })
    return () => window.removeEventListener('popstate', onPopState, { capture: true })
  }, [providerSettingsDirty, updateProviderDirty])

  const overview = useQuery({
    queryKey: ['system-overview'],
    queryFn: getSystemOverview,
    refetchInterval: 30_000,
  })
  const transfers = useQuery({
    queryKey: ['transfers'],
    queryFn: () => listTransfers(50),
    refetchInterval: 5_000,
  })

  const integrations = overview.data?.integrations ?? []
  const connectedCount = integrations.filter((item) => item.status === 'healthy').length
  const configuredCount = integrations.filter((item) => item.status !== 'unconfigured').length
  const connectionLabel = overview.isLoading
    ? '正在检查服务'
    : connectedCount > 0
      ? `${connectedCount} 个服务在线`
      : configuredCount > 0 ? '服务当前不可用' : '尚未配置服务'
  const connectionState = connectedCount > 0 ? 'healthy' : configuredCount > 0 ? 'degraded' : 'unconfigured'
  const activeTransferCount = transfers.data?.transfers.filter((job) => activeTransferStates.has(job.state)).length ?? 0
  const systemActive = activeView === '运维' || activeView === '服务'

  const navigate = (next: WorkspaceView) => {
    if (next === activeView) return true
    if (activeView === '服务' && providerSettingsDirty) {
      if (!window.confirm('Provider 设置尚未保存。确定离开并放弃修改吗？')) return false
      updateProviderDirty(false)
    }
    commitUrl({
      view: viewSlugs[next],
      q: next === '发现' ? new URLSearchParams(window.location.search).get('q') : null,
      task: next === '任务' ? new URLSearchParams(window.location.search).get('task') : null,
      settings: next === '服务' ? new URLSearchParams(window.location.search).get('settings') : null,
    })
    setActiveView(next)
    window.scrollTo({ top: 0 })
    return true
  }
  const guardedLogout = () => {
    if (activeView === '服务' && providerSettingsDirty && !window.confirm('Provider 设置尚未保存。确定退出并放弃修改吗？')) return
    updateProviderDirty(false)
    onLogout()
  }
  const handleNav = (event: MouseEvent<HTMLAnchorElement>, next: WorkspaceView) => {
    if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return
    event.preventDefault()
    navigate(next)
  }

  const navLink = ({ label, icon: Icon }: (typeof mediaNavItems)[number] | (typeof systemNavItems)[number]) => (
    <a
      aria-current={activeView === label ? 'page' : undefined}
      className={activeView === label ? 'nav-item active' : 'nav-item'}
      href={`?view=${viewSlugs[label]}`}
      key={label}
      onClick={(event) => handleNav(event, label)}
    >
      <Icon size={18} />
      <span>{label === '服务' ? '系统设置' : label}</span>
      {label === '任务' && activeTransferCount > 0 ? <span className="nav-count">{activeTransferCount}</span> : null}
    </a>
  )

  return (
    <div className="app-shell">
      <a className="skip-link" href="#main-content">跳到主要内容</a>
      <aside className="sidebar">
        <div className="brand-lockup">
          <div className="brand-mark"><Film size={19} strokeWidth={2.2} /></div>
          <div><strong>MEDIA HUB</strong><span>PRIVATE MEDIA CONTROL</span></div>
        </div>

        <nav className="primary-nav" aria-label="主导航">
          <div className="nav-group"><div className="sidebar-section-label">媒体</div>{mediaNavItems.map(navLink)}</div>
          <div className="nav-group system-group"><div className="sidebar-section-label">系统</div>{systemNavItems.map(navLink)}</div>
        </nav>

        <div className="sidebar-bottom">
          <div className={`connection-summary ${connectionState}`}><span className="connection-pulse" /><span>{connectionLabel}</span></div>
          <button className="profile-row" disabled={isLoggingOut} onClick={guardedLogout} type="button">
            <span className="avatar">W</span>
            <span className="profile-copy"><strong>管理员</strong><small>{isLoggingOut ? '正在退出' : '本地账户'}</small></span>
            <LogOut aria-hidden="true" size={15} />
          </button>
        </div>
      </aside>

      <main className="main-content" id="main-content" tabIndex={-1}>
        <header className="topbar">
          <div className="topbar-title"><span>Media Hub</span><strong>{activeView === '服务' ? '系统设置' : activeView}</strong></div>
          <div className={`system-dot ${connectionState}`}><span />{connectionLabel}</div>
          <a
            aria-label="系统设置"
            className={systemActive ? 'topbar-system-link active' : 'topbar-system-link'}
            href="?view=settings"
            onClick={(event) => handleNav(event, '服务')}
          >
            <Settings2 size={18} />
          </a>
        </header>

        {systemActive ? (
          <nav className="system-subnav" aria-label="系统导航">
            {systemNavItems.map(({ label, icon: Icon }) => <a aria-current={activeView === label ? 'page' : undefined} href={`?view=${viewSlugs[label]}`} key={label} onClick={(event) => handleNav(event, label)}><Icon size={16} />{label === '服务' ? '系统设置' : label}</a>)}
          </nav>
        ) : null}

        <div className="page-wrap">
          {activeView === '发现' ? (
            <DiscoveryView
              integrations={integrations}
              integrationsLoading={overview.isLoading}
              onRefreshIntegrations={() => void overview.refetch()}
              onTransferCreated={() => navigate('任务')}
              onSubscribe={(candidate) => { setDraftSubscription(candidate); navigate('订阅') }}
            />
          ) : null}
          {activeView === '任务' ? <TransferQueue query={transfers} /> : null}
          {activeView === '订阅' ? <SubscriptionView draftCandidate={draftSubscription} onDraftConsumed={() => setDraftSubscription(null)} /> : null}
          {activeView === '媒体库' ? <LibraryView /> : null}
          {activeView === '运维' ? <OperationsView /> : null}
          {activeView === '服务' ? <SettingsView integrations={integrations} onDirtyChange={updateProviderDirty} onLogout={guardedLogout} onRefresh={() => void overview.refetch()} /> : null}
        </div>
      </main>

      <nav className="mobile-nav" aria-label="移动端主导航">
        {mediaNavItems.map(navLink)}
      </nav>
    </div>
  )
}
