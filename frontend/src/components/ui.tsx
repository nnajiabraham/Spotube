import type { ReactNode, SVGProps } from 'react'
import { YoutubeLogo } from './YoutubeLogo'

// Shared building blocks so every page has the same width, header, and card styling.

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
    <main className={`mx-auto px-4 py-8 sm:px-6 lg:px-8 ${width === 'wide' ? 'max-w-7xl' : 'max-w-3xl'}`}>
      <div className="mb-6 flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-gray-900">{title}</h1>
          {description && <p className="mt-1 text-sm text-gray-500">{description}</p>}
        </div>
        {actions && <div className="flex items-center gap-2">{actions}</div>}
      </div>
      {children}
    </main>
  )
}

export function Card({ className = '', children }: { className?: string; children: ReactNode }) {
  return <div className={`rounded-xl border border-gray-200 bg-white ${className}`}>{children}</div>
}

export function SectionTitle({ children, action }: { children: ReactNode; action?: ReactNode }) {
  return (
    <div className="mb-3 flex items-center justify-between">
      <h2 className="text-sm font-semibold uppercase tracking-wide text-gray-500">{children}</h2>
      {action}
    </div>
  )
}


const pillTones = {
  gray: 'bg-gray-100 text-gray-700 ring-gray-200',
  green: 'bg-green-50 text-green-700 ring-green-200',
  yellow: 'bg-amber-50 text-amber-700 ring-amber-200',
  red: 'bg-red-50 text-red-700 ring-red-200',
  blue: 'bg-blue-50 text-blue-700 ring-blue-200',
}

export type PillTone = keyof typeof pillTones

export function Pill({ tone = 'gray', children }: { tone?: PillTone; children: ReactNode }) {
  return (
    <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ring-1 ring-inset ${pillTones[tone]}`}>
      {children}
    </span>
  )
}

const queueStatusTone: Record<string, PillTone> = {
  pending: 'yellow',
  running: 'blue',
  done: 'green',
  error: 'red',
  skipped: 'gray',
}

const queueStatusLabel: Record<string, string> = {
  pending: 'Pending',
  running: 'Running',
  done: 'Synced',
  error: 'Failed',
  skipped: 'Skipped',
}

export function QueueStatusPill({ status }: { status: string }) {
  return <Pill tone={queueStatusTone[status] ?? 'gray'}>{queueStatusLabel[status] ?? status}</Pill>
}

export function SpotifyLogo(props: SVGProps<SVGSVGElement>) {
  return (
    <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true" {...props}>
      <path d="M12 0C5.4 0 0 5.4 0 12s5.4 12 12 12 12-5.4 12-12S18.66 0 12 0zm5.52 17.34c-.24.36-.66.48-1.02.24-2.82-1.74-6.36-2.1-10.56-1.14-.42.12-.78-.18-.9-.54-.12-.42.18-.78.54-.9 4.56-1.02 8.52-.6 11.64 1.32.42.18.48.66.3 1.02zm1.44-3.3c-.3.42-.84.6-1.26.3-3.24-1.98-8.16-2.58-11.94-1.38-.48.12-1.02-.12-1.14-.6-.12-.48.12-1.02.6-1.14C9.6 9.9 15 10.56 18.72 12.84c.36.18.54.78.24 1.2zm.12-3.36C15.24 8.4 8.82 8.16 5.16 9.3c-.6.18-1.2-.18-1.38-.72-.18-.6.18-1.2.72-1.38 4.26-1.26 11.28-1.02 15.72 1.62.54.3.72 1.02.42 1.56-.3.42-1.02.6-1.56.3z" />
    </svg>
  )
}

export function ServiceIcon({ service, className = 'h-4 w-4' }: { service: string; className?: string }) {
  if (service === 'spotify') return <SpotifyLogo className={`${className} text-[#1DB954]`} />
  if (service === 'youtube') return <YoutubeLogo className={`${className} text-[#FF0000]`} />
  return null
}
