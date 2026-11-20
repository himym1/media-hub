import { useEffect, useRef } from 'react'
import { CircleAlert } from 'lucide-react'

export type ConfirmDialogProps = {
  open: boolean
  title: string
  message: string
  confirmLabel?: string
  cancelLabel?: string
  onConfirm: () => void
  onCancel: () => void
}

/** 页内确认弹窗，替代容易被 Tauri / WebView2 吞掉的原生 window.confirm。 */
export function ConfirmDialog({
  open,
  title,
  message,
  confirmLabel = '放弃修改',
  cancelLabel = '继续编辑',
  onConfirm,
  onCancel,
}: ConfirmDialogProps) {
  const cancelRef = useRef<HTMLButtonElement>(null)
  const previousFocusRef = useRef<HTMLElement | null>(null)

  useEffect(() => {
    if (open) {
      previousFocusRef.current = document.activeElement as HTMLElement | null
      cancelRef.current?.focus()
    } else if (previousFocusRef.current) {
      previousFocusRef.current.focus()
      previousFocusRef.current = null
    }
  }, [open])

  useEffect(() => {
    if (!open) return
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault()
        onCancel()
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [open, onCancel])

  if (!open) return null

  return (
    <div
      aria-describedby="confirm-dialog-message"
      aria-labelledby="confirm-dialog-title"
      aria-modal="true"
      className="confirm-dialog-backdrop"
      onClick={(event) => {
        if (event.target === event.currentTarget) onCancel()
      }}
      role="alertdialog"
    >
      <div className="confirm-dialog">
        <div className="confirm-dialog-header">
          <CircleAlert aria-hidden="true" className="confirm-dialog-icon" size={18} />
          <h2 id="confirm-dialog-title">{title}</h2>
        </div>
        <p className="confirm-dialog-message" id="confirm-dialog-message">{message}</p>
        <div className="confirm-dialog-actions">
          <button
            className="secondary-command"
            onClick={onCancel}
            ref={cancelRef}
            type="button"
          >
            {cancelLabel}
          </button>
          <button
            className="danger-button"
            onClick={onConfirm}
            type="button"
          >
            {confirmLabel}
          </button>
        </div>
      </div>
    </div>
  )
}
