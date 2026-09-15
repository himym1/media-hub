import { useEffect, useRef } from 'react'
import { Command, Compass, Navigation, X } from 'lucide-react'

export type ShortcutsModalProps = {
  isOpen: boolean
  onClose: () => void
}

interface ShortcutEntry {
  keys: string[]
  description: string
}

interface ShortcutSection {
  title: string
  icon: typeof Command
  items: ShortcutEntry[]
}

const shortcutSections: ShortcutSection[] = [
  {
    title: '全局与导航',
    icon: Navigation,
    items: [
      { keys: ['⌘', 'K'], description: '唤出全局命令面板 (Windows 使用 Ctrl+K)' },
      { keys: ['/'], description: '快速聚焦搜索输入框' },
      { keys: ['?'], description: '随时唤出本快捷键帮助' },
      { keys: ['Esc'], description: '关闭当前浮层、抽屉或命令面板' },
    ],
  },
  {
    title: '影视检索与选片',
    icon: Compass,
    items: [
      { keys: ['J', '↓'], description: '切换至下一个候选版本（自动平滑滚动）' },
      { keys: ['K', '↑'], description: '切换至上一个候选版本' },
      { keys: ['Enter'], description: '在命令面板中执行高亮指令' },
    ],
  },
  {
    title: '无障碍与交互',
    icon: Command,
    items: [
      { keys: ['Tab'], description: '在可交互控件间顺序移动焦点' },
      { keys: ['Shift', 'Tab'], description: '反向移动交互焦点' },
      { keys: ['Space'], description: '触发当前按钮或展开可折叠项' },
    ],
  },
]

export function ShortcutsModal({ isOpen, onClose }: ShortcutsModalProps) {
  const closeButtonRef = useRef<HTMLButtonElement>(null)
  const previousFocusRef = useRef<HTMLElement | null>(null)

  useEffect(() => {
    if (isOpen) {
      previousFocusRef.current = document.activeElement as HTMLElement | null
      closeButtonRef.current?.focus()
    } else if (previousFocusRef.current) {
      previousFocusRef.current.focus()
    }
  }, [isOpen])

  useEffect(() => {
    if (!isOpen) return

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault()
        onClose()
      }
    }

    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [isOpen, onClose])

  if (!isOpen) return null

  return (
    <div
      aria-label="键盘快捷键指南"
      aria-modal="true"
      className="shortcuts-backdrop"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose()
      }}
      role="dialog"
    >
      <div className="shortcuts-modal">
        <header className="shortcuts-header">
          <div className="shortcuts-title">
            <Command aria-hidden="true" size={18} />
            <h2>键盘快捷键速查</h2>
          </div>
          <button
            aria-label="关闭快捷键指南"
            className="shortcuts-close-btn"
            onClick={onClose}
            ref={closeButtonRef}
            type="button"
          >
            <X size={16} />
          </button>
        </header>

        <div className="shortcuts-body">
          {shortcutSections.map((section) => {
            const Icon = section.icon
            return (
              <section className="shortcuts-section" key={section.title}>
                <div className="shortcuts-section-heading">
                  <Icon size={14} />
                  <h3>{section.title}</h3>
                </div>
                <div className="shortcuts-grid">
                  {section.items.map((item) => (
                    <div className="shortcuts-row" key={item.description}>
                      <span className="shortcuts-desc">{item.description}</span>
                      <div className="shortcuts-keys">
                        {item.keys.map((key) => (
                          <kbd className="shortcuts-key" key={key}>
                            {key}
                          </kbd>
                        ))}
                      </div>
                    </div>
                  ))}
                </div>
              </section>
            )
          })}
        </div>

        <footer className="shortcuts-footer">
          <span>提示：在任意页面键入 <kbd>?</kbd> 即可随时查阅</span>
        </footer>
      </div>
    </div>
  )
}
