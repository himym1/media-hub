import { createContext, useContext } from 'react'

export type ToastType = 'success' | 'info' | 'error'

export interface ToastItem {
  id: string
  title: string
  type: ToastType
}

export interface ToastContextValue {
  showToast: (title: string, type?: ToastType) => void
}

export const ToastContext = createContext<ToastContextValue | null>(null)

export function useToast(): ToastContextValue {
  const ctx = useContext(ToastContext)
  if (!ctx) {
    return {
      showToast: () => {},
    }
  }
  return ctx
}
