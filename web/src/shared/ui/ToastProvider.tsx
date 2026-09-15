import { useCallback, useMemo, useState, type ReactNode } from 'react'
import { AlertCircle, CheckCircle2, Info } from 'lucide-react'
import { ToastContext, type ToastItem, type ToastType } from './ToastContext'

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<ToastItem[]>([])

  const showToast = useCallback((title: string, type: ToastType = 'success') => {
    const id = `${Date.now()}-${Math.random().toString(36).slice(2, 7)}`
    setToasts((prev) => [...prev.slice(-3), { id, title, type }])
    setTimeout(() => {
      setToasts((prev) => prev.filter((item) => item.id !== id))
    }, 2800)
  }, [])

  const value = useMemo(() => ({ showToast }), [showToast])

  return (
    <ToastContext.Provider value={value}>
      {children}
      <div
        aria-atomic="true"
        aria-label="操作反馈提示"
        aria-live="polite"
        className="toast-container"
        role="status"
      >
        {toasts.map((toast) => {
          const Icon =
            toast.type === 'error'
              ? AlertCircle
              : toast.type === 'info'
                ? Info
                : CheckCircle2
          return (
            <div className={`toast-bubble ${toast.type}`} key={toast.id}>
              <span aria-hidden="true" className="toast-bubble-icon">
                <Icon size={16} />
              </span>
              <span className="toast-bubble-text">{toast.title}</span>
            </div>
          )
        })}
      </div>
    </ToastContext.Provider>
  )
}
