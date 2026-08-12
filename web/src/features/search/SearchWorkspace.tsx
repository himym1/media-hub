import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  ChevronRight,
  Film,
  LayoutGrid,
  LibraryBig,
  ListPlus,
  ListTodo,
  LogOut,
  Settings2,
  TerminalSquare,
} from 'lucide-react'
import { getSystemOverview, listTransfers, type Candidate } from '../../shared/api/mediaHub'
import { LibraryView } from '../library/LibraryView'
import { OperationsView } from '../operations/OperationsView'
import { SettingsView } from '../settings/SettingsView'
import { SubscriptionView } from '../subscriptions/SubscriptionView'
import { TransferQueue } from '../transfers/TransferQueue'
import { DiscoveryView } from './DiscoveryView'
import './SearchWorkspace.css'

type WorkspaceView = '发现' | '订阅' | '任务' | '媒体库' | '运维' | '服务'

const navItems = [
  { label: '发现', icon: LayoutGrid },
  { label: '订阅', icon: ListPlus },
  { label: '任务', icon: ListTodo },
  { label: '媒体库', icon: LibraryBig },
  { label: '运维', icon: TerminalSquare },
  { label: '服务', icon: Settings2 },
] satisfies { label: WorkspaceView; icon: typeof LayoutGrid }[]

const activeTransferStates = new Set(['queued', 'transferring', 'transferred', 'submitting_sync', 'syncing', 'retry_wait', 'refreshing_emby', 'indexing_emby', 'verifying_playback'])

type SearchWorkspaceProps = {
  isLoggingOut: boolean
  onLogout: () => void
}

export function SearchWorkspace({ isLoggingOut, onLogout }: SearchWorkspaceProps) {
  const [activeView, setActiveView] = useState<WorkspaceView>('发现')
  const [draftSubscription, setDraftSubscription] = useState<Candidate | null>(null)

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

  const connectedCount = overview.data?.integrations.filter((item) => item.status === 'healthy').length ?? 0
  const activeTransferCount = transfers.data?.transfers.filter((job) => activeTransferStates.has(job.state)).length ?? 0

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="brand-lockup">
          <div className="brand-mark"><Film size={19} strokeWidth={2.4} /></div>
          <div>
            <strong>MEDIA HUB</strong>
            <span>HOME MEDIA CONTROL</span>
          </div>
        </div>

        <div className="sidebar-section-label">工作区</div>
        <nav className="primary-nav" aria-label="主导航">
          {navItems.map(({ label, icon: Icon }) => (
            <button
              aria-current={activeView === label ? 'page' : undefined}
              className={activeView === label ? 'nav-item active' : 'nav-item'}
              key={label}
              onClick={() => setActiveView(label)}
              type="button"
            >
              <Icon size={18} />
              <span>{label}</span>
              {label === '任务' && activeTransferCount > 0 ? <span className="nav-count">{activeTransferCount}</span> : null}
            </button>
          ))}
        </nav>

        <div className="sidebar-bottom">
          <div className="sidebar-section-label">连接状态</div>
          <div className="connection-summary">
            <span className="connection-pulse" />
            <span>{connectedCount > 0 ? `${connectedCount} 个服务在线` : '等待连接服务'}</span>
          </div>
          <button className="profile-row" disabled={isLoggingOut} onClick={onLogout} type="button">
            <span className="avatar">W</span>
            <span className="profile-copy"><strong>管理员</strong><small>{isLoggingOut ? '正在退出' : '本地账户'}</small></span>
            <LogOut aria-hidden="true" size={15} />
          </button>
        </div>
      </aside>

      <main className="main-content">
        <header className="topbar">
          <div className="breadcrumb"><span>工作区</span><ChevronRight size={14} /><strong>{activeView}</strong></div>
          <div className={connectedCount > 0 ? 'system-dot' : 'system-dot degraded'}>
            <span />{connectedCount > 0 ? '服务已连接' : '服务未连接'}
          </div>
        </header>

        <div className="page-wrap">
          {activeView === '发现' ? (
            <DiscoveryView
              integrations={overview.data?.integrations ?? []}
              integrationsLoading={overview.isLoading}
              onRefreshIntegrations={() => void overview.refetch()}
              onTransferCreated={() => setActiveView('任务')}
              onSubscribe={(candidate) => {
                setDraftSubscription(candidate)
                setActiveView('订阅')
              }}
            />
          ) : null}
          {activeView === '订阅' ? <SubscriptionView draftCandidate={draftSubscription} onDraftConsumed={() => setDraftSubscription(null)} /> : null}
          {activeView === '任务' ? <TransferQueue query={transfers} /> : null}
          {activeView === '媒体库' ? <LibraryView /> : null}
          {activeView === '运维' ? <OperationsView /> : null}
          {activeView === '服务' ? <SettingsView integrations={overview.data?.integrations ?? []} onRefresh={() => void overview.refetch()} /> : null}
          <footer className="page-footer"><span>MEDIA HUB / CONTROL PLANE</span><span>API v0.5 · NAS LOCAL</span></footer>
        </div>
      </main>
    </div>
  )
}
