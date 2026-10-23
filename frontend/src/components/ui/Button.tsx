import type { ButtonHTMLAttributes, ReactNode } from 'react'
import { Loader2 } from 'lucide-react'
import { buttonClass, type ButtonVariant } from '../../lib/format'

/**
 * Button with the shared variants: primary (ink), secondary (outlined), ghost, danger
 * (YouTube red) and spotify (green, for "connect"/"confirm match" actions).
 */
export function Button({
  variant = 'secondary',
  size = 'md',
  loading = false,
  icon,
  className = '',
  children,
  disabled,
  type = 'button',
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: ButtonVariant
  size?: 'sm' | 'md'
  loading?: boolean
  icon?: ReactNode
}) {
  const sizeClass = size === 'sm' ? '!px-2.5 !py-1.5 !text-xs' : ''
  return (
    <button
      type={type}
      className={`${buttonClass[variant]} ${sizeClass} ${className}`}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
      {...props}
    >
      {loading ? <Loader2 className="h-3.5 w-3.5 animate-spin" aria-hidden="true" /> : icon}
      {children}
    </button>
  )
}
