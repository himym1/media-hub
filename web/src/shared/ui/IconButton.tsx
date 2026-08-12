import type { ButtonHTMLAttributes, ReactNode } from 'react'

type IconButtonProps = {
  children: ReactNode
  label: string
  subtle?: boolean
} & Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'aria-label' | 'children' | 'className' | 'type'>

export function IconButton({ children, label, subtle = false, ...buttonProps }: IconButtonProps) {
  return (
    <button
      aria-label={label}
      className={subtle ? 'icon-button subtle' : 'icon-button'}
      type="button"
      {...buttonProps}
    >
      {children}
    </button>
  )
}
