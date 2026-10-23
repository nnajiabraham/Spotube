import type { ReactNode } from 'react'
import { AlertTriangle, CheckCircle2, Info, XCircle } from 'lucide-react'

// Page scaffolding shared by every screen: same width, header, card and message styling.

export function Page({
  title,
  description,
  actions,
  width = 'wide',
  children,
}: {
  title: string
  description?: ReactNode
  actions?: ReactNode
  width?: 'wide' | 'narrow'
  children: ReactNode
}) {
  return (
    <main className={`mx-auto px-4 pb-12 pt-8 sm:px-6 lg:px-8 ${width === 'wide' ? 'max-w-7xl' : 'max-w-3xl'}`}>
      <div className="mb-6 flex flex-wrap items-end justify-between gap-4 border-b border-gray-200 pb-5">
        <div className="min-w-0">
          <h1 className="flex items-baseline gap-2 text-2xl font-semibold tracking-tight text-ink">
            <span className="select-none text-gray-300" aria-hidden="true">
              /
            </span>
            {title}
          </h1>
          {description && <div className="mt-1.5 max-w-3xl text-sm leading-relaxed text-gray-500">{description}</div>}
        </div>
        {actions && <div className="flex items-center gap-2">{actions}</div>}
      </div>
      {children}
    </main>
  )
}

export function Card({ className = '', children }: { className?: string; children: ReactNode }) {
  return <div className={`rounded-lg border border-gray-200 bg-white ${className}`}>{children}</div>
}

export function SectionTitle({ children, action }: { children: ReactNode; action?: ReactNode }) {
  return (
    <div className="mb-3 flex items-center justify-between">
      <h2 className="text-[11px] font-semibold uppercase tracking-[0.14em] text-gray-500">{children}</h2>
      {action}
    </div>
  )
}

/** Centered message for an empty list, with an optional icon and call to action. */
export function EmptyState({
  icon,
  title,
  children,
  action,
}: {
  icon?: ReactNode
  title: string
  children?: ReactNode
  action?: ReactNode
}) {
  return (
    <div className="flex flex-col items-center px-6 py-12 text-center">
      {icon && <div className="mb-3 text-gray-300 [&>svg]:h-10 [&>svg]:w-10">{icon}</div>}
      <p className="font-medium text-ink">{title}</p>
      {children && <div className="mt-1 max-w-md text-sm text-gray-500">{children}</div>}
      {action && <div className="mt-4">{action}</div>}
    </div>
  )
}

const noticeTones = {
  success: { box: 'border-spotify-100 bg-spotify-50 text-spotify-700', Icon: CheckCircle2 },
  error: { box: 'border-youtube-100 bg-youtube-50 text-youtube-700', Icon: XCircle },
  warning: { box: 'border-amber-200 bg-amber-50 text-amber-800', Icon: AlertTriangle },
  info: { box: 'border-gray-200 bg-white text-gray-700', Icon: Info },
}

export type NoticeTone = keyof typeof noticeTones

/** Inline message banner. Errors are announced to screen readers immediately. */
export function Notice({
  tone = 'info',
  children,
  onDismiss,
  className = '',
}: {
  tone?: NoticeTone
  children: ReactNode
  onDismiss?: () => void
  className?: string
}) {
  const { box, Icon } = noticeTones[tone]
  return (
    <div
      role={tone === 'error' ? 'alert' : 'status'}
      className={`flex items-start gap-2.5 rounded-md border px-3.5 py-2.5 text-sm ${box} ${className}`}
    >
      <Icon className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
      <div className="min-w-0 flex-1">{children}</div>
      {onDismiss && (
        <button type="button" onClick={onDismiss} className="text-xs font-medium opacity-70 hover:opacity-100">
          Dismiss
        </button>
      )}
    </div>
  )
}

/** Label + value pair used in detail panels. */
export function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div>
      <dt className="text-[11px] font-medium uppercase tracking-wider text-gray-400">{label}</dt>
      <dd className="mt-1 break-words text-ink">{children}</dd>
    </div>
  )
}
