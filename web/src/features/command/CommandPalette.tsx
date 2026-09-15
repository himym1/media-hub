import { useEffect, useMemo, useRef, useState, type ComponentType } from 'react'
import {
  CornerDownLeft,
  Keyboard,
  LayoutGrid,
  LibraryBig,
  ListPlus,
  ListTodo,
  LogOut,
  RefreshCw,
  Search,
  Settings2,
  TerminalSquare,
  X,
} from 'lucide-react'

export type WorkspaceView = '发现' | '任务' | '订阅' | '媒体库' | '运维' | '服务'

interface CommandItem {
  id: string
  category: '导航' | '常用操作' | '搜索'
  title: string
  subtitle?: string
  icon: ComponentType<{ size?: number; className?: string }>
  action: () => void
  keywords?: string[]
}

export type CommandPaletteProps = {
  isOpen: boolean
  onClose: () => void
  onNavigate: (view: WorkspaceView) => void
  onSearch: (query: string) => void
  onRefreshIntegrations: () => void
  onLogout: () => void
  onOpenShortcuts?: () => void
}

export function CommandPalette({
  isOpen,
  onClose,
  onNavigate,
  onSearch,
  onRefreshIntegrations,
  onLogout,
  onOpenShortcuts,
}: CommandPaletteProps) {
  const [query, setQuery] = useState('')
  const [selectedIndex, setSelectedIndex] = useState(0)
  const inputRef = useRef<HTMLInputElement>(null)
  const listRef = useRef<HTMLDivElement>(null)
  const previousFocusRef = useRef<HTMLElement | null>(null)

  useEffect(() => {
    if (isOpen) {
      previousFocusRef.current = document.activeElement as HTMLElement | null
      setQuery('')
      setSelectedIndex(0)
      const timer = setTimeout(() => {
        inputRef.current?.focus()
      }, 50)
      return () => clearTimeout(timer)
    } else if (previousFocusRef.current) {
      previousFocusRef.current.focus()
    }
  }, [isOpen])

  const baseCommands: CommandItem[] = useMemo(
    () => [
      {
        id: 'nav-discover',
        category: '导航',
        title: '影视发现与检索',
        subtitle: '搜索影视资源、热门精选与高分榜单',
        icon: LayoutGrid,
        action: () => onNavigate('发现'),
        keywords: ['discover', 'search', 'movie', 'tv', 'tmdb', 'faxian', 'sousuo'],
      },
      {
        id: 'nav-transfers',
        category: '导航',
        title: '传输与转存任务',
        subtitle: '查看 115 转存队列、STRM 与 Emby 刮削进度',
        icon: ListTodo,
        action: () => onNavigate('任务'),
        keywords: ['transfers', 'tasks', 'queue', 'download', '115', 'renwu'],
      },
      {
        id: 'nav-subscriptions',
        category: '导航',
        title: '追番与更新订阅',
        subtitle: '管理 Mikan 追番规则与自动更新流水线',
        icon: ListPlus,
        action: () => onNavigate('订阅'),
        keywords: ['subscriptions', 'anime', 'mikan', 'dingyue', 'zhuifan'],
      },
      {
        id: 'nav-library',
        category: '导航',
        title: 'Emby 媒体库浏览',
        subtitle: '浏览 NAS Emby 影视库',
        icon: LibraryBig,
        action: () => onNavigate('媒体库'),
        keywords: ['library', 'emby', 'meitiku', 'shipin'],
      },
      {
        id: 'nav-operations',
        category: '导航',
        title: '系统运维与日志',
        subtitle: '查看系统健康状态、服务日志与归档管理',
        icon: TerminalSquare,
        action: () => onNavigate('运维'),
        keywords: ['operations', 'ops', 'logs', 'yunwei', 'rizhi'],
      },
      {
        id: 'nav-settings',
        category: '导航',
        title: '系统与集成设置',
        subtitle: '配置 115 账号、Emby 密钥、字幕源与偏好',
        icon: Settings2,
        action: () => onNavigate('服务'),
        keywords: ['settings', 'config', 'token', 'shezhi', 'fuwu'],
      },
      {
        id: 'action-refresh',
        category: '常用操作',
        title: '刷新服务集成状态',
        subtitle: '重新探测 115、Emby、OpenSubtitles 等服务连通性',
        icon: RefreshCw,
        action: onRefreshIntegrations,
        keywords: ['refresh', 'status', 'health', 'shuaxin', 'zhuangtai'],
      },
      {
        id: 'action-logout',
        category: '常用操作',
        title: '安全退出登录',
        subtitle: '退出当前 Media Hub 管理员会话',
        icon: LogOut,
        action: onLogout,
        keywords: ['logout', 'signout', 'exit', 'tuichu'],
      },
      ...(onOpenShortcuts ? [{
        id: 'action-shortcuts',
        category: '常用操作' as const,
        title: '键盘快捷键速查指南',
        subtitle: '浏览完整快捷键列表（随时按 ? 键唤出）',
        icon: Keyboard,
        action: onOpenShortcuts,
        keywords: ['shortcuts', 'help', 'keyboard', 'kuaijiejian', 'bangzhu', '?'],
      }] : []),
    ],
    [onLogout, onNavigate, onOpenShortcuts, onRefreshIntegrations],
  )

  const filteredCommands = useMemo(() => {
    const trimmed = query.trim()
    const result: CommandItem[] = []

    const lower = trimmed.toLowerCase()
    const matched = baseCommands.filter((cmd) => {
      if (!lower) return true
      if (cmd.title.toLowerCase().includes(lower)) return true
      if (cmd.subtitle?.toLowerCase().includes(lower)) return true
      if (cmd.keywords?.some((k) => k.toLowerCase().includes(lower))) return true
      return false
    })

    result.push(...matched)

    if (trimmed) {
      result.push({
        id: 'instant-search',
        category: '搜索',
        title: `在发现中检索「${trimmed}」`,
        subtitle: '按 Enter 立即搜索该片源',
        icon: Search,
        action: () => onSearch(trimmed),
        keywords: [],
      })
    }

    return result
  }, [baseCommands, onSearch, query])

  useEffect(() => {
    setSelectedIndex(0)
  }, [query])

  useEffect(() => {
    if (!isOpen) return

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault()
        onClose()
      } else if (e.key === 'ArrowDown') {
        e.preventDefault()
        setSelectedIndex((prev) => (filteredCommands.length > 0 ? (prev + 1) % filteredCommands.length : 0))
      } else if (e.key === 'ArrowUp') {
        e.preventDefault()
        setSelectedIndex((prev) => (filteredCommands.length > 0 ? (prev - 1 + filteredCommands.length) % filteredCommands.length : 0))
      } else if (e.key === 'Enter') {
        e.preventDefault()
        const selected = filteredCommands[selectedIndex]
        if (selected) {
          selected.action()
          onClose()
        }
      }
    }

    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [filteredCommands, isOpen, onClose, selectedIndex])

  // Scroll active item into view
  useEffect(() => {
    if (!isOpen || !listRef.current) return
    const activeEl = listRef.current.querySelector<HTMLElement>(`[data-command-index="${selectedIndex}"]`)
    if (activeEl) {
      activeEl.scrollIntoView({ block: 'nearest' })
    }
  }, [isOpen, selectedIndex])

  if (!isOpen) return null

  return (
    <div
      aria-label="全局命令面板"
      aria-modal="true"
      className="command-palette-backdrop"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose()
      }}
      role="dialog"
    >
      <div className="command-palette-modal">
        <div className="command-palette-search-bar">
          <Search aria-hidden="true" className="command-palette-search-icon" size={18} />
          <input
            aria-autocomplete="list"
            aria-controls="command-palette-list"
            aria-label="搜索影视或键入操作指令"
            className="command-palette-input"
            onChange={(e) => setQuery(e.target.value)}
            placeholder="搜索影视或键入操作指令..."
            ref={inputRef}
            type="text"
            value={query}
          />
          {query ? (
            <button
              aria-label="清空输入"
              className="command-palette-clear-btn"
              onClick={() => {
                setQuery('')
                inputRef.current?.focus()
              }}
              type="button"
            >
              <X size={15} />
            </button>
          ) : null}
          <button
            aria-label="关闭命令面板"
            className="command-palette-close-btn"
            onClick={onClose}
            type="button"
          >
            <kbd>Esc</kbd>
          </button>
        </div>

        <div aria-label="可用命令建议" className="command-palette-list" id="command-palette-list" ref={listRef} role="listbox">
          {filteredCommands.length === 0 ? (
            <div className="command-palette-empty">
              <span>没有找到与「{query}」相关的命令</span>
            </div>
          ) : (
            filteredCommands.map((cmd, idx) => {
              const isSelected = idx === selectedIndex
              const Icon = cmd.icon
              return (
                <button
                  aria-selected={isSelected}
                  className={isSelected ? 'command-palette-item selected' : 'command-palette-item'}
                  data-command-index={idx}
                  key={cmd.id}
                  onClick={() => {
                    cmd.action()
                    onClose()
                  }}
                  onMouseEnter={() => setSelectedIndex(idx)}
                  role="option"
                  type="button"
                >
                  <div className="command-item-left">
                    <span className="command-item-icon">
                      <Icon size={16} />
                    </span>
                    <div className="command-item-copy">
                      <strong className="command-item-title">{cmd.title}</strong>
                      {cmd.subtitle ? <span className="command-item-sub">{cmd.subtitle}</span> : null}
                    </div>
                  </div>
                  <div className="command-item-right">
                    <span className="command-item-badge">{cmd.category}</span>
                    {isSelected ? <CornerDownLeft aria-hidden="true" className="command-item-enter" size={14} /> : null}
                  </div>
                </button>
              )
            })
          )}
        </div>

        <footer className="command-palette-footer">
          <div className="command-palette-hints">
            <span><kbd>↑</kbd><kbd>↓</kbd> 移动光标</span>
            <span><kbd>↵</kbd> 执行</span>
            <span><kbd>Esc</kbd> 关闭</span>
          </div>
          <span className="command-palette-brand">Media Hub Quick Command</span>
        </footer>
      </div>
    </div>
  )
}
