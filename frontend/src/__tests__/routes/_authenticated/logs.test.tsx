import { describe, it, expect, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import { Route } from '../../../routes/_authenticated/logs.lazy';

vi.mock('@tanstack/react-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@tanstack/react-router')>();
  return {
    ...actual,
    Link: ({ to, children, className }: { to: string; children: React.ReactNode; className?: string }) => (
      <a href={to} className={className}>
        {children}
      </a>
    ),
  };
});

const ActivityLogsPage = (Route.options.component ?? (() => null)) as React.ComponentType;

const renderWithProviders = (ui: React.ReactElement) => {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>);
};

const mockActivityLogs = {
  page: 1,
  perPage: 50,
  totalItems: 3,
  totalPages: 1,
  items: [
    {
      id: 'log1',
      level: 'info' as const,
      message: 'Starting sync analysis job',
      mapping_id: 'mapping1',
      job_type: 'analysis' as const,
      created: 1704110400,
    },
    {
      id: 'log2',
      level: 'info' as const,
      message: 'Added "Test Song" to YouTube',
      sync_item_id: 'sync_item_1',
      job_type: 'executor' as const,
      created: 1704110700,
      details_json: {
        version: 1,
        kind: 'executor_run',
        data: { outcome: 'added', sync_item_id: 'sync_item_1' },
      },
    },
    {
      id: 'log3',
      level: 'error' as const,
      message: 'Sync failed: reconnect required (spotify): oauth2: "invalid_grant"',
      mapping_id: 'mapping1',
      job_type: 'analysis' as const,
      created: 1704111000,
    },
  ],
};

const mockSyncItem = {
  id: 'sync_item_1',
  mapping_id: 'mapping1',
  service: 'youtube',
  operation: 'add',
  status: 'done',
  track_id: 'spotify_track_1',
  track_title: 'Test Song',
  track_artist: 'Test Artist',
  attempt_count: 1,
  created: 1704109000,
  updated: 1704111000,
};

const server = globalThis.mswServer;

const serveLogs = (handler?: (url: URL) => object) =>
  server?.use(
    http.get('*/api/collections/activity_logs/records', ({ request }) =>
      HttpResponse.json(handler?.(new URL(request.url)) ?? mockActivityLogs),
    ),
  );

describe('Activity page', () => {
  it('lists log entries with their job type and mapping name', async () => {
    serveLogs();
    renderWithProviders(<ActivityLogsPage />);

    expect(screen.getByRole('heading', { name: 'Activity' })).toBeInTheDocument();
    expect(await screen.findByText('Starting sync analysis job')).toBeInTheDocument();
    expect(screen.getByText('Added "Test Song" to YouTube')).toBeInTheDocument();
    expect(screen.getByText('Track copy')).toBeInTheDocument();
    expect(screen.getAllByText('Sync run')).toHaveLength(2);
    await waitFor(() => {
      expect(screen.getAllByText('My Spotify Playlist ⇄ My YouTube Playlist')).toHaveLength(2);
    });
    expect(screen.getByText('3 entries')).toBeInTheDocument();
  });

  it('links reconnect errors to the dashboard', async () => {
    serveLogs();
    renderWithProviders(<ActivityLogsPage />);

    const link = await screen.findByRole('link', { name: 'Reconnect Spotify' });
    expect(link).toHaveAttribute('href', '/dashboard');
  });

  it('filters by tab', async () => {
    const user = userEvent.setup();
    const seen: string[] = [];
    serveLogs((url) => {
      seen.push(url.search);
      if (url.searchParams.get('level') === 'error') {
        return { ...mockActivityLogs, totalItems: 1, items: [mockActivityLogs.items[2]] };
      }
      if (url.searchParams.get('job_type') === 'executor') {
        return { ...mockActivityLogs, totalItems: 1, items: [mockActivityLogs.items[1]] };
      }
      return mockActivityLogs;
    });
    renderWithProviders(<ActivityLogsPage />);
    await screen.findByText('Starting sync analysis job');

    await user.click(screen.getByRole('tab', { name: 'Errors' }));
    await waitFor(() => {
      expect(screen.queryByText('Starting sync analysis job')).not.toBeInTheDocument();
    });
    expect(screen.getByText(/Sync failed: reconnect required/)).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: 'Errors' })).toHaveAttribute('aria-selected', 'true');

    await user.click(screen.getByRole('tab', { name: 'Track copies' }));
    await waitFor(() => {
      expect(screen.queryByText(/Sync failed/)).not.toBeInTheDocument();
    });
    expect(screen.getByText('Added "Test Song" to YouTube')).toBeInTheDocument();
    expect(seen.some((s) => s.includes('job_type=executor'))).toBe(true);
  });

  it('opens the sync item drawer from "View track" and closes it', async () => {
    const user = userEvent.setup();
    serveLogs();
    server?.use(
      http.get('*/api/collections/sync_items/records/sync_item_1', () => HttpResponse.json(mockSyncItem)),
    );
    renderWithProviders(<ActivityLogsPage />);

    await user.click(await screen.findByRole('button', { name: 'View track' }));
    const dialog = await screen.findByRole('dialog');
    await waitFor(() => expect(dialog).toHaveTextContent('Test Song'));

    await user.click(screen.getByRole('button', { name: 'Close' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
  });

  it('expands details', async () => {
    const user = userEvent.setup();
    serveLogs();
    renderWithProviders(<ActivityLogsPage />);

    await user.click(await screen.findByRole('button', { name: /details/i }));
    expect(screen.getByText(/executor_run/)).toBeInTheDocument();
    expect(screen.getByText(/"outcome": "added"/)).toBeInTheDocument();
  });

  it('shows loading, error and empty states', async () => {
    server?.use(
      http.get('*/api/collections/activity_logs/records', () =>
        HttpResponse.json({ error: 'Internal server error' }, { status: 500 }),
      ),
    );
    const { unmount } = renderWithProviders(<ActivityLogsPage />);
    expect(screen.getByRole('status')).toHaveTextContent('Loading activity');
    expect(await screen.findByText(/Couldn't load activity/)).toBeInTheDocument();
    unmount();

    serveLogs(() => ({ page: 1, perPage: 50, totalItems: 0, totalPages: 0, items: [] }));
    renderWithProviders(<ActivityLogsPage />);
    expect(await screen.findByText('Nothing has run yet.')).toBeInTheDocument();
  });

  it('refetches when Refresh is clicked', async () => {
    const user = userEvent.setup();
    let calls = 0;
    serveLogs(() => {
      calls++;
      return mockActivityLogs;
    });
    renderWithProviders(<ActivityLogsPage />);
    await screen.findByText('Starting sync analysis job');
    expect(calls).toBe(1);

    await user.click(screen.getByRole('button', { name: /refresh/i }));
    await waitFor(() => expect(calls).toBe(2));
  });
});
