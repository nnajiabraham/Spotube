import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { DashboardStatsCards } from './DashboardStatsCards'
import type { DashboardStats } from '../lib/api/index'

vi.mock('@tanstack/react-router', () => ({
  Link: ({ children, to, search }: { children: React.ReactNode; to: string; search?: { status?: string } }) => (
    <a href={search?.status ? `${to}?status=${search.status}` : to}>{children}</a>
  ),
}))

const stats: DashboardStats = {
  mappings: { total: 2 },
  queue: { pending: 12, running: 0, done: 1024, error: 2, skipped: 3, review: 7 },
  recent_runs: [],
  youtube_quota: { used: 2500, limit: 10000 },
}

describe('DashboardStatsCards', () => {
  it('shows each queue count and links it to the filtered queue', () => {
    render(<DashboardStatsCards stats={stats} isLoading={false} />)

    expect(screen.getByText('12')).toBeInTheDocument()
    expect(screen.getByText('1,024')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /failed/i })).toHaveAttribute('href', '/sync-queue?status=error')
    expect(screen.getByRole('link', { name: /pending/i })).toHaveAttribute('href', '/sync-queue?status=pending')
  })

  it('links the review count to the review page', () => {
    render(<DashboardStatsCards stats={stats} isLoading={false} />)

    const review = screen.getByRole('link', { name: /needs review/i })
    expect(review).toHaveAttribute('href', '/review')
    expect(review).toHaveTextContent('7')
  })

  it('shows measured YouTube quota use against the daily limit', () => {
    render(<DashboardStatsCards stats={stats} isLoading={false} />)

    expect(screen.getByText('YouTube quota')).toBeInTheDocument()
    expect(screen.getByText('2,500')).toBeInTheDocument()
    expect(screen.getByLabelText('25% of daily quota')).toBeInTheDocument()
  })

  it('shows placeholders while loading', () => {
    render(<DashboardStatsCards isLoading />)

    expect(screen.getAllByRole('status').length).toBeGreaterThan(0)
  })
})
