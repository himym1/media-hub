import { useCallback, useEffect, useRef, useState, type MouseEvent } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Compass, Keyboard, LibraryBig, ListPlus, ListTodo, LogOut, Search, Settings2 } from 'lucide-react'
import { getSystemOverview, listTransfers, type Candidate } from '../../shared/api/mediaHub'
import {
  DESKTOP_LOGOUT_EVENT,
  DESKTOP_NAVIGATE_EVENT,
  isDesktopShell,
  primaryShortcutModifier,
  workspaceViewFromDesktopEvent,
} from '../../shared/desktop/desktopShell'
import { useDesktopUpdate } from '../../shared/desktop/useDesktopUpdate'
import { commitUrl } from '../../shared/navigation/urlState'
import { BrandMark } from '../../shared/ui/BrandMark'
import { ConfirmDialog } from '../../shared/ui/ConfirmDialog'
import { useToast } from '../../shared/ui/ToastContext'
import { ToastProvider } from '../../shared/ui/ToastProvider'
import { CommandPalette } from '../command/CommandPalette'
import { ShortcutsModal } from '../command/ShortcutsModal'
import { LibraryView } from '../library/LibraryView'
import { PlayerShell } from '../library/PlayerShell'
import { isPlayerView } from '../library/playerRoute'
import { OperationsView } from '../operations/OperationsView'
import { DesktopUpdateBanner } from '../settings/DesktopUpdatePanel'
import { SettingsView } from '../settings/SettingsView'
import { SubscriptionView } from '../subscriptions/SubscriptionView'
import { TransferQueue } from '../transfers/TransferQueue'
import { DiscoveryView } from './DiscoveryView'
import './SearchWorkspace.css'

type WorkspaceView = '发现' | '任务' | '订阅' | '媒体库' | '运维' | '服务'

type PendingLeave = {
  message: string
  confirmLabel: string
  onConfirm: () => void
}

const mediaNavItems = [
  { label: '发现', icon: Compass },
  { label: '任务', icon: ListTodo },
  { label: '订阅', icon: ListPlus },
  { label: '媒体库', icon: LibraryBig },
] satisfies { label: WorkspaceView; icon: typeof Compass }[]

const systemNavItems = [
  { label: '服务', icon: Settings2 },
] satisfies { label: WorkspaceView; icon: typeof Compass }[]

const viewSlugs: Record<WorkspaceView, string> = {
  发现: 'discover',
  任务: 'transfers',
  订阅: 'subscriptions',
  媒体库: 'library',
  运维: 'operations',
  服务: 'settings',
}
const slugViews = Object.fromEntries(Object.entries(viewSlugs).map(([view, slug]) => [slug, view])) as Record<string, WorkspaceView>
const shortcutViewKeys: Record<string, WorkspaceView> = {
  '1': '发现',
  '2': '任务',
  '3': '订阅',
  '4': '媒体库',
  ',': '服务',
}
const activeTransferStates = new Set(['queued', 'transferring', 'downloading', 'transferred', 'submitting_sync', 'syncing', 'retry_wait', 'refreshing_emby', 'indexing_emby', 'verifying_playback'])

type SearchWorkspaceProps = {
  isLoggingOut: boolean
  onLogout: () => void
}

function viewFromLocation(): WorkspaceView {
  return slugViews[new URLSearchParams(window.location.search).get('view') ?? ''] ?? '发现'
}

export function SearchWorkspace(props: SearchWorkspaceProps) {
  if (isPlayerView()) return <PlayerShell />
  return (
    <ToastProvider>
      <WorkspaceShell {...props} />
    </ToastProvider>
  )
}

function WorkspaceShell({ isLoggingOut, onLogout }: SearchWorkspaceProps) {
  const desktopUpdate = useDesktopUpdate()
  const { showToast } = useToast()
  const desktopShell = isDesktopShell()
  const shortcutMod = primaryShortcutModifier()
  const commandShortcut = shortcutMod === '⌘' ? '⌘K' : `${shortcutMod}+K`
  const [activeView, setActiveView] = useState<WorkspaceView>(viewFromLocation)
  const [mountedViews, setMountedViews] = useState<Set<WorkspaceView>>(() => new Set([viewFromLocation()]))
  const [draftSubscription, setDraftSubscription] = useState<Candidate | null>(null)
  const [commandOpen, setCommandOpen] = useState(false)
  const [shortcutsOpen, setShortcutsOpen] = useState(false)
  const [pendingLeave, setPendingLeave] = useState<PendingLeave | null>(null)
  const providerDirtyRef = useRef(false)
  const providerDirtyUrl = useRef<string | null>(null)
  const updateProviderDirty = useCallback((dirty: boolean) => {
    providerDirtyRef.current = dirty
    if (dirty && providerDirtyUrl.current === null) providerDirtyUrl.current = window.location.href
    if (!dirty) providerDirtyUrl.current = null
  }, [])
  const isProviderDirty = useCallback(() => providerDirtyRef.current, [])
  const requestLeaveConfirm = useCallback((message: string, onConfirm: () => void, confirmLabel = '放弃修改') => {
    setPendingLeave({ message, confirmLabel, onConfirm })
  }, [])

  const handleGlobalSearch = useCallback((searchQuery: string) => {
    commitUrl({
      view: 'discover',
      q: searchQuery,
      task: null,
      archive: null,
      subscription: null,
      library: null,
      media: null,
      settings: null,
    })
    setActiveView('发现')
    window.dispatchEvent(new PopStateEvent('popstate'))
  }, [])

  useEffect(() => {
    setMountedViews((prev) => {
      if (prev.has(activeView)) return prev
      const next = new Set(prev)
      next.add(activeView)
      return next
    })
  }, [activeView])

  useEffect(() => {
    const onPopState = (event: PopStateEvent) => {
      if (providerDirtyRef.current && providerDirtyUrl.current && window.location.href !== providerDirtyUrl.current) {
        const targetUrl = window.location.href
        window.history.pushState({}, '', providerDirtyUrl.current)
        event.stopImmediatePropagation()
        requestLeaveConfirm('Provider 设置尚未保存。确定离开并放弃修改吗？', () => {
          updateProviderDirty(false)
          window.history.replaceState({}, '', targetUrl)
          setActiveView(viewFromLocation())
          window.dispatchEvent(new PopStateEvent('popstate'))
        })
        return
      }
      setActiveView(viewFromLocation())
    }
    window.addEventListener('popstate', onPopState, { capture: true })
    return () => window.removeEventListener('popstate', onPopState, { capture: true })
  }, [requestLeaveConfirm, updateProviderDirty])

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

  const integrations = (overview.data?.integrations ?? []).filter((item) => item.id !== 'qmediasync')
  const connectedCount = integrations.filter((item) => item.status === 'healthy').length
  const configuredCount = integrations.filter((item) => item.status !== 'unconfigured').length
  const connectionLabel = overview.isLoading && integrations.length === 0
    ? '正在检查服务'
    : connectedCount > 0
      ? `${connectedCount} 个服务在线`
      : configuredCount > 0 ? '服务当前不可用' : '尚未配置服务'
  const connectionState = connectedCount > 0 ? 'healthy' : configuredCount > 0 ? 'degraded' : 'unconfigured'
  const activeTransferCount = transfers.data?.transfers.filter((job) => activeTransferStates.has(job.state)).length ?? 0
  const systemActive = activeView === '服务'

  const applyNavigate = useCallback((next: WorkspaceView) => {
    updateProviderDirty(false)
    const current = new URLSearchParams(window.location.search)
    commitUrl({
      view: viewSlugs[next],
      q: next === '发现' ? current.get('q') : null,
      task: next === '任务' ? current.get('task') : null,
      archive: next === '任务' ? current.get('archive') : null,
      subscription: next === '订阅' ? current.get('subscription') : null,
      library: next === '媒体库' ? current.get('library') : null,
      media: next === '媒体库' ? current.get('media') : null,
      settings: next === '服务' ? current.get('settings') : null,
    })
    setActiveView(next)
  }, [updateProviderDirty])

  const navigate = useCallback((next: WorkspaceView) => {
    if (next === activeView) return true
    if (activeView === '服务' && providerDirtyRef.current) {
      requestLeaveConfirm('Provider 设置尚未保存。确定离开并放弃修改吗？', () => applyNavigate(next))
      return false
    }
    applyNavigate(next)
    return true
  }, [activeView, applyNavigate, requestLeaveConfirm])

  const guardedLogout = useCallback(() => {
    if (activeView === '服务' && providerDirtyRef.current) {
      requestLeaveConfirm(
        'Provider 设置尚未保存。确定退出并放弃修改吗？',
        () => {
          updateProviderDirty(false)
          onLogout()
        },
        '放弃并退出',
      )
      return
    }
    updateProviderDirty(false)
    onLogout()
  }, [activeView, onLogout, requestLeaveConfirm, updateProviderDirty])

  useEffect(() => {
    if (!desktopShell) return
    const label = activeView === '服务' ? '系统设置' : activeView
    document.title = `${label} — Media Hub`
  }, [activeView, desktopShell])

  useEffect(() => {
    if (!desktopShell) return
    const onNavigate = (event: Event) => {
      const slug = workspaceViewFromDesktopEvent(event)
      const next = slug ? slugViews[slug] : undefined
      if (!next) return
      navigate(next)
    }
    const onDesktopLogout = () => guardedLogout()
    window.addEventListener(DESKTOP_NAVIGATE_EVENT, onNavigate)
    window.addEventListener(DESKTOP_LOGOUT_EVENT, onDesktopLogout)
    return () => {
      window.removeEventListener(DESKTOP_NAVIGATE_EVENT, onNavigate)
      window.removeEventListener(DESKTOP_LOGOUT_EVENT, onDesktopLogout)
    }
  }, [desktopShell, guardedLogout, navigate])

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (pendingLeave) return
      if ((e.metaKey || e.ctrlKey) && !e.altKey && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        setCommandOpen((prev) => !prev)
        return
      }
      if ((e.metaKey || e.ctrlKey) && !e.altKey && !e.shiftKey && !commandOpen && !shortcutsOpen) {
        const mappedView = shortcutViewKeys[e.key]
        if (mappedView) {
          e.preventDefault()
          navigate(mappedView)
          return
        }
      }
      if (e.key === '/' && !commandOpen && !shortcutsOpen) {
        const target = e.target as HTMLElement | null
        if (target && !['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName) && !target.isContentEditable) {
          e.preventDefault()
          if (activeView !== '发现') navigate('发现')
          window.requestAnimationFrame(() => {
            document.getElementById('media-search')?.focus()
          })
          return
        }
      }
      if ((e.key === '?' || (e.shiftKey && e.key === '/')) && !commandOpen && !shortcutsOpen) {
        const target = e.target as HTMLElement | null
        if (target && !['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName) && !target.isContentEditable) {
          e.preventDefault()
          setShortcutsOpen(true)
          return
        }
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [activeView, commandOpen, navigate, pendingLeave, shortcutsOpen])

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
      {desktopShell ? <div aria-hidden="true" className="desktop-drag-region" data-tauri-drag-region /> : null}
      <a className="skip-link" href="#main-content">跳到主要内容</a>
      <aside className="sidebar">
        <div className="brand-lockup">
          <div className="brand-mark"><BrandMark size={20} /></div>
          <div><strong>MEDIA HUB</strong><span>媒体自动化控制</span></div>
        </div>

        {desktopShell ? (
          <div className="desktop-sidebar-brand">
            <span className="desktop-brand-mark"><BrandMark size={15} /></span>
            <strong>Media Hub</strong>
          </div>
        ) : null}

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

        {desktopShell ? (
          <div className="desktop-sidebar-footer">
            <button
              aria-label={`打开命令面板 (${commandShortcut})`}
              className="desktop-command-btn"
              onClick={() => setCommandOpen(true)}
              type="button"
            >
              <Search aria-hidden="true" size={14} />
              <span>快速跳转</span>
              <kbd>{commandShortcut}</kbd>
            </button>
            <div className="desktop-sidebar-status-row">
              <span className={`desktop-status-pill ${connectionState}`} title={connectionLabel}>
                <span aria-hidden="true" className="desktop-status-dot" />
                <span>{connectionLabel}</span>
              </span>
              <button
                aria-label="快捷键指南"
                className="desktop-shortcuts-btn"
                onClick={() => setShortcutsOpen(true)}
                title="快捷键指南 (?)"
                type="button"
              >
                <Keyboard aria-hidden="true" size={15} />
              </button>
            </div>
          </div>
        ) : null}
      </aside>

      <main className="main-content" id="main-content" tabIndex={-1}>
        <header className="topbar">
          <div className="topbar-title"><span>Media Hub</span><strong>{activeView === '服务' ? '系统设置' : activeView}</strong></div>
          <button
            aria-label={`打开命令面板 (快捷键 ${commandShortcut})`}
            className="command-trigger-btn"
            onClick={() => setCommandOpen(true)}
            type="button"
          >
            <Search size={14} />
            <span className="command-trigger-label">跳转或搜索…</span>
            <kbd className="command-trigger-kbd">{commandShortcut}</kbd>
          </button>
          <div className="topbar-right">
            <div className={`system-dot ${connectionState}`}><span />{connectionLabel}</div>
            <a
              aria-label="系统设置"
              className={systemActive ? 'topbar-system-link active' : 'topbar-system-link'}
              href="?view=settings"
              onClick={(event) => handleNav(event, '服务')}
            >
              <Settings2 size={18} />
            </a>
          </div>
        </header>

        <DesktopUpdateBanner update={desktopUpdate} />
        <div className="page-wrap">
          {mountedViews.has('发现') ? (
            <div aria-hidden={activeView !== '发现'} className="page-pane" hidden={activeView !== '发现'}>
              <DiscoveryView
                onTransferCreated={() => {
                  showToast('已加入任务，可在「任务」查看进度', 'success')
                }}
                onSubscribe={(candidate) => {
                  showToast(`已将《${candidate.title}》载入追番配置`, 'info')
                  setDraftSubscription(candidate)
                  navigate('订阅')
                }}
              />
            </div>
          ) : null}
          {mountedViews.has('任务') ? (
            <div aria-hidden={activeView !== '任务'} className="page-pane" hidden={activeView !== '任务'}>
              <TransferQueue query={transfers} />
            </div>
          ) : null}
          {mountedViews.has('订阅') ? (
            <div aria-hidden={activeView !== '订阅'} className="page-pane" hidden={activeView !== '订阅'}>
              <SubscriptionView draftCandidate={draftSubscription} onDraftConsumed={() => setDraftSubscription(null)} />
            </div>
          ) : null}
          {mountedViews.has('媒体库') ? (
            <div aria-hidden={activeView !== '媒体库'} className="page-pane" hidden={activeView !== '媒体库'}>
              <LibraryView />
            </div>
          ) : null}
          {mountedViews.has('运维') ? (
            <div aria-hidden={activeView !== '运维'} className="page-pane" hidden={activeView !== '运维'}>
              <OperationsView />
            </div>
          ) : null}
          {mountedViews.has('服务') ? (
            <div aria-hidden={activeView !== '服务'} className="page-pane" hidden={activeView !== '服务'}>
              <SettingsView
                desktopUpdate={desktopUpdate}
                integrations={integrations}
                isProviderDirty={isProviderDirty}
                onConfirmLeave={(message, onConfirm) => requestLeaveConfirm(message, onConfirm)}
                onDirtyChange={updateProviderDirty}
                onLogout={guardedLogout}
                onRefresh={() => void overview.refetch()}
              />
            </div>
          ) : null}
        </div>

        <CommandPalette
          isOpen={commandOpen}
          onClose={() => setCommandOpen(false)}
          onLogout={guardedLogout}
          onNavigate={(view) => {
            navigate(view)
          }}
          onOpenShortcuts={() => setShortcutsOpen(true)}
          onRefreshIntegrations={() => {
            void overview.refetch()
            showToast('已刷新服务状态', 'success')
          }}
          onSearch={handleGlobalSearch}
        />
        <ShortcutsModal
          isOpen={shortcutsOpen}
          onClose={() => setShortcutsOpen(false)}
        />
        <ConfirmDialog
          confirmLabel={pendingLeave?.confirmLabel}
          message={pendingLeave?.message ?? ''}
          onCancel={() => setPendingLeave(null)}
          onConfirm={() => {
            const action = pendingLeave?.onConfirm
            setPendingLeave(null)
            action?.()
          }}
          open={Boolean(pendingLeave)}
          title="尚未保存的更改"
        />
      </main>

      <nav className="mobile-nav" aria-label="移动端主导航">
        {mediaNavItems.map(navLink)}
      </nav>
    </div>
  )
}
