import { describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { http, HttpResponse } from 'msw'
import { Route } from '../../../routes/settings/index.lazy'
import { mockSettings } from '../../../test/mocks/handlers'

vi.mock('@tanstack/react-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@tanstack/react-router')>()
  return {
    ...actual,
    Link: ({ to, children, className }: { to: string; children: React.ReactNode; className?: string }) => (
      <a href={to} className={className}>
        {children}
      </a>
    ),
  }
})

const SettingsPage = Route.options.component as React.ComponentType

const renderPage = () =>
  render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <SettingsPage />
    </QueryClientProvider>,
  )

describe('Settings page', () => {
  it('groups settings and shows where each value comes from', async () => {
    renderPage()
    expect(await screen.findByText('Scheduled syncs')).toBeInTheDocument()
    expect(screen.getByText('Sync')).toBeInTheDocument()
    expect(screen.getByText('YouTube quota')).toBeInTheDocument()
    // .env-pinned setting says changes only last until restart
    expect(screen.getByText('.env')).toBeInTheDocument()
    expect(screen.getByText(/Changes here last until the server restarts/)).toBeInTheDocument()
    // quota panel
    expect(await screen.findByText(/units used today/)).toBeInTheDocument()
  })

  it('toggles a switch and saves a number', async () => {
    const sent: Record<string, unknown>[] = []
    globalThis.mswServer?.use(
      http.patch('*/api/settings', async ({ request }) => {
        const body = (await request.json()) as Record<string, unknown>
        sent.push(body)
        return HttpResponse.json({
          items: mockSettings.map((s) =>
            s.key in body ? { ...s, value: String(body[s.key]), source: s.env_pinned ? 'runtime' : 'database' } : s,
          ),
        })
      }),
    )
    const user = userEvent.setup()
    renderPage()

    const workers = await screen.findByRole('switch', { name: 'Scheduled syncs' })
    expect(workers).toBeChecked()
    await user.click(workers)
    await waitFor(() => expect(sent).toEqual([{ sync_workers_enabled: false }]))
    expect(await screen.findByText('until restart')).toBeInTheDocument()
    expect(screen.getByRole('switch', { name: 'Scheduled syncs' })).not.toBeChecked()

    const reserve = screen.getByRole('spinbutton', { name: 'Units kept for you' })
    const row = reserve.closest('form') as HTMLElement
    expect(within(row).getByRole('button', { name: 'Save' })).toBeDisabled()
    await user.clear(reserve)
    await user.type(reserve, '800')
    await user.click(within(row).getByRole('button', { name: 'Save' }))
    await waitFor(() => expect(sent[1]).toEqual({ youtube_interactive_reserve: 800 }))
    expect(await screen.findByText('saved')).toBeInTheDocument()
  })

  it('shows the server error when a value is refused', async () => {
    globalThis.mswServer?.use(
      http.patch('*/api/settings', () =>
        HttpResponse.json({ message: 'sync_new_mappings_now: must be true or false' }, { status: 422 }),
      ),
    )
    const user = userEvent.setup()
    renderPage()
    await user.click(await screen.findByRole('switch', { name: 'Start new mappings straight away' }))
    expect(await screen.findByRole('alert')).toHaveTextContent(/must be true or false/)
  })
})
