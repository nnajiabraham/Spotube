import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import { Route } from '../../../routes/_authenticated/sync-queue.lazy';

const navigate = vi.fn();
let search: { status?: string } = {};

vi.mock('@tanstack/react-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@tanstack/react-router')>();
  return {
    ...actual,
    Link: ({ to, children, className }: { to: string; children: React.ReactNode; className?: string }) => (
      <a href={to} className={className}>
        {children}
      </a>
    ),
    useNavigate: () => navigate,
    useSearch: () => search,
  };
});

const SyncQueuePage = (Route.options.component ?? (() => null)) as React.ComponentType;

const renderWithProviders = (ui: React.ReactElement) => {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>);
};

const baseItem = {
  mapping_id: 'mapping1',
  operation: 'add',
  service: 'youtube',
  attempt_count: 0,
  created: 1704107200,
  updated: 1704107200,
  source_service: 'spotify',
  destination_service: 'youtube',
  source_playlist_name: 'Road Trip',
  destination_playlist_name: 'YT Road Trip',
};

function listResponse(items: Array<Record<string, unknown>>) {
  return HttpResponse.json({ page: 1, perPage: 25, totalItems: items.length, totalPages: 1, items });
}

describe('Sync Queue Page', () => {
  const server = globalThis.mswServer;

  beforeEach(() => {
    vi.clearAllMocks();
    search = {};
  });

  it('lists tracks with destination playlist and status reason', async () => {
    server?.use(
      http.get('*/api/collections/sync_items/records', () =>
        listResponse([
          { ...baseItem, id: 'a', track_title: 'Neon Dreams', track_artist: 'Artist X', status: 'pending' },
          {
            ...baseItem,
            id: 'b',
            track_title: 'Fearless',
            status: 'error',
            attempt_count: 3,
            error_message: 'no confident match on youtube (best score 0.65)',
          },
        ]),
      ),
    );

    renderWithProviders(<SyncQueuePage />);

    expect(await screen.findByText('Neon Dreams')).toBeInTheDocument();
    expect(screen.getByText('Artist X')).toBeInTheDocument();
    expect(screen.getAllByText('YT Road Trip')).toHaveLength(2);
    expect(screen.getByText('no confident match on youtube (best score 0.65)')).toBeInTheDocument();
    expect(screen.getByText('3 attempts')).toBeInTheDocument();
    expect(screen.getByText('2 items')).toBeInTheDocument();
  });

  it('passes the status from the URL to the API and updates it from the tabs', async () => {
    const user = userEvent.setup();
    search = { status: 'error' };
    let requestedStatus: string | null = null;
    server?.use(
      http.get('*/api/collections/sync_items/records', ({ request }) => {
        requestedStatus = new URL(request.url).searchParams.get('status');
        return listResponse([]);
      }),
    );

    renderWithProviders(<SyncQueuePage />);

    await waitFor(() => expect(requestedStatus).toBe('error'));
    expect(screen.getByRole('tab', { name: 'Failed' })).toHaveAttribute('aria-selected', 'true');

    await user.click(screen.getByRole('tab', { name: 'Synced' }));
    expect(navigate).toHaveBeenCalledWith({ search: { status: 'done' } });
  });

  it('runs a pending item and reports the result', async () => {
    const user = userEvent.setup();
    let executeCalls = 0;
    server?.use(
      http.get('*/api/collections/sync_items/records', () =>
        listResponse([{ ...baseItem, id: 'p1', track_title: 'Song', status: 'pending' }]),
      ),
      http.post('*/api/collections/sync_items/records/p1/execute', () => {
        executeCalls += 1;
        return HttpResponse.json({ ...baseItem, id: 'p1', track_title: 'Song', status: 'done', attempt_count: 1 });
      }),
    );

    renderWithProviders(<SyncQueuePage />);

    await user.click(await screen.findByRole('button', { name: /run now/i }));

    await waitFor(() => {
      expect(executeCalls).toBe(1);
      expect(screen.getByRole('alert')).toHaveTextContent('Song: synced.');
    });
  });

  it('offers no action for finished items', async () => {
    server?.use(
      http.get('*/api/collections/sync_items/records', () =>
        listResponse([{ ...baseItem, id: 'd1', track_title: 'Done Song', status: 'done' }]),
      ),
    );

    renderWithProviders(<SyncQueuePage />);

    await screen.findByText('Done Song');
    expect(screen.queryByRole('button', { name: /run now|retry/i })).not.toBeInTheDocument();
  });

  it('opens a details drawer with the analysis decision and executor candidates', async () => {
    const user = userEvent.setup();
    server?.use(
      http.get('*/api/collections/sync_items/records', () =>
        listResponse([{ ...baseItem, id: 'x1', track_title: 'Umbrella', track_artist: 'Moses Bliss', status: 'done' }]),
      ),
      http.get('*/api/collections/sync_items/records/x1', () =>
        HttpResponse.json({
          ...baseItem,
          id: 'x1',
          track_id: 'sp-umbrella',
          track_title: 'Umbrella',
          track_artist: 'Moses Bliss',
          status: 'done',
          analysis_context_json: {
            version: 1,
            decision: { best_match: { platform: 'youtube', id: 'v0', title: 'Other Song' }, score: 0.2, matched: false },
          },
        }),
      ),
      http.get('*/api/collections/activity_logs/records', () =>
        HttpResponse.json({
          page: 1,
          perPage: 10,
          totalItems: 1,
          totalPages: 1,
          items: [
            {
              id: 'log1',
              level: 'info',
              job_type: 'executor',
              message: 'executor add to youtube',
              created: 1704107300,
              details_json: {
                data: {
                  search_query: 'Umbrella Moses Bliss',
                  outcome: 'added',
                  selected: { id: 'vid1' },
                  candidates: [{ rank: 1, id: 'vid1', title: 'Umbrella', artist: 'Moses Bliss', channel: 'Moses Bliss', score: 1 }],
                },
              },
            },
          ],
        }),
      ),
    );

    renderWithProviders(<SyncQueuePage />);

    await user.click(await screen.findByText('Umbrella'));

    const drawer = await screen.findByRole('dialog');
    expect(await within(drawer).findByText('Why it was queued')).toBeInTheDocument();
    expect(within(drawer).getByText('0.20')).toBeInTheDocument();
    expect(await within(drawer).findByText('“Umbrella Moses Bliss”')).toBeInTheDocument();
    const candidate = within(drawer).getAllByRole('link', { name: /umbrella — moses bliss/i });
    expect(candidate.some((a) => a.getAttribute('href') === 'https://www.youtube.com/watch?v=vid1')).toBe(true);
    expect(within(drawer).getByText('1.00')).toBeInTheDocument();
  });
});
