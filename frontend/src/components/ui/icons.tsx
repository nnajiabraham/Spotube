import type { SVGProps } from 'react'
import { YoutubeLogo } from './YoutubeLogo'

export { YoutubeLogo }

export function SpotifyLogo(props: SVGProps<SVGSVGElement>) {
  return (
    <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true" {...props}>
      <path d="M12 0C5.4 0 0 5.4 0 12s5.4 12 12 12 12-5.4 12-12S18.66 0 12 0zm5.52 17.34c-.24.36-.66.48-1.02.24-2.82-1.74-6.36-2.1-10.56-1.14-.42.12-.78-.18-.9-.54-.12-.42.18-.78.54-.9 4.56-1.02 8.52-.6 11.64 1.32.42.18.48.66.3 1.02zm1.44-3.3c-.3.42-.84.6-1.26.3-3.24-1.98-8.16-2.58-11.94-1.38-.48.12-1.02-.12-1.14-.6-.12-.48.12-1.02.6-1.14C9.6 9.9 15 10.56 18.72 12.84c.36.18.54.78.24 1.2zm.12-3.36C15.24 8.4 8.82 8.16 5.16 9.3c-.6.18-1.2-.18-1.38-.72-.18-.6.18-1.2.72-1.38 4.26-1.26 11.28-1.02 15.72 1.62.54.3.72 1.02.42 1.56-.3.42-1.02.6-1.56.3z" />
    </svg>
  )
}

/** Brand mark for a service, in its brand colour. */
export function ServiceIcon({ service, className = 'h-4 w-4' }: { service: string; className?: string }) {
  if (service === 'spotify') return <SpotifyLogo className={`${className} text-spotify-600`} />
  if (service === 'youtube') return <YoutubeLogo className={`${className} text-youtube`} />
  return null
}

/** The Spotube mark: green arrow out (Spotify), red arrow back (YouTube) on ink. */
export function SpotubeMark({ className = 'h-7 w-7' }: { className?: string }) {
  return (
    <svg viewBox="0 0 32 32" className={className} aria-hidden="true">
      <rect width="32" height="32" rx="8" fill="#0f0f0f" />
      <path d="M9 12h12m0 0-3.5-3.5M21 12l-3.5 3.5" fill="none" stroke="#1ed760" strokeWidth="2.6" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M23 20H11m0 0 3.5-3.5M11 20l3.5 3.5" fill="none" stroke="#ff0033" strokeWidth="2.6" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}
