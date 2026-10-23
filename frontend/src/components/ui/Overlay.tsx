import { useEffect, useId, type ReactNode } from 'react'
import { X } from 'lucide-react'

function useEscape(onClose: () => void) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])
}

function Header({ id, title, subtitle, onClose }: { id: string; title: ReactNode; subtitle?: ReactNode; onClose: () => void }) {
  return (
    <div className="flex items-start justify-between gap-4 border-b border-gray-200 px-5 py-3.5">
      <div className="min-w-0">
        <h2 id={id} className="truncate text-base font-semibold text-ink">
          {title}
        </h2>
        {subtitle && <div className="mt-0.5 text-sm text-gray-500">{subtitle}</div>}
      </div>
      <button type="button" onClick={onClose} className="rounded-md p-1 text-gray-400 hover:bg-gray-100 hover:text-ink" aria-label="Close">
        <X className="h-5 w-5" />
      </button>
    </div>
  )
}

/** Panel that slides in from the right; closes on Escape or a click outside. */
export function Drawer({
  title,
  subtitle,
  onClose,
  children,
  footer,
}: {
  title: ReactNode
  subtitle?: ReactNode
  onClose: () => void
  children: ReactNode
  footer?: ReactNode
}) {
  const id = useId()
  useEscape(onClose)
  return (
    <div className="fixed inset-0 z-50 flex justify-end bg-ink/25 backdrop-blur-[2px]" onClick={onClose}>
      <aside
        role="dialog"
        aria-modal="true"
        aria-labelledby={id}
        className="flex h-full w-full max-w-2xl flex-col border-l border-gray-200 bg-white shadow-xl"
        onClick={(e) => e.stopPropagation()}
      >
        <Header id={id} title={title} subtitle={subtitle} onClose={onClose} />
        <div className="flex-1 overflow-y-auto px-5 py-4 text-sm">{children}</div>
        {footer && <div className="border-t border-gray-200 px-5 py-3">{footer}</div>}
      </aside>
    </div>
  )
}

/** Centered modal dialog; closes on Escape or a click outside. */
export function Dialog({
  title,
  subtitle,
  onClose,
  children,
  footer,
  size = 'md',
}: {
  title: ReactNode
  subtitle?: ReactNode
  onClose: () => void
  children: ReactNode
  footer?: ReactNode
  size?: 'md' | 'lg'
}) {
  const id = useId()
  useEscape(onClose)
  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto bg-ink/25 p-4 backdrop-blur-[2px] sm:pt-16" onClick={onClose}>
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby={id}
        className={`flex max-h-[85vh] w-full flex-col overflow-hidden rounded-lg border border-gray-200 bg-white shadow-xl ${size === 'lg' ? 'max-w-3xl' : 'max-w-xl'}`}
        onClick={(e) => e.stopPropagation()}
      >
        <Header id={id} title={title} subtitle={subtitle} onClose={onClose} />
        <div className="flex-1 overflow-y-auto px-5 py-4 text-sm">{children}</div>
        {footer && <div className="flex justify-end gap-2 border-t border-gray-200 bg-gray-50 px-5 py-3">{footer}</div>}
      </div>
    </div>
  )
}
