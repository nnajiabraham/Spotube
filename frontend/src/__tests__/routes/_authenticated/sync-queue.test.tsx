import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import { Route } from '../../../routes/_authenticated/sync-queue.lazy';

vi.mock('@tanstack/react-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@tanstack/react-router')>();
  return {
    ...actual,
    Link: ({
      to,
      children,
      className,
    }: {
      to: string;
      children: React.ReactNode;
      className?: string;
    }) => (
      <a href={to} className={className}>
        {children}
      </a>
    ),
    useNavigate: () => vi.fn(),
  };
});

const SyncQueuePage = (Route.options.component ?? (() => null)) as React.ComponentType;

const renderWithProviders = (ui: React.ReactElement) => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });

  return render(
    <QueryClientProvider client={queryClient}>
      {ui}
    </QueryClientProvider>,
  );
};

describe('Sync Queue Page', () => {
  const server = globalThis.mswServer;

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders table with source → dest and playlist names', async () => {
    server?.use(
      http.get('*/api/collections/sync_items/records', () => {
        return HttpResponse.json({
          page: 1,
          perPage: 50,
          totalItems: 1,
          totalPages: 1,
          items: [
            {
              id: 'sync-pending-1',
              mapping_id: 'mapping1',
              operation: 'add',
              service: 'youtube',
              track_id: 'sp-1',
              track_title: 'Neon Dreams',
              track_artist: 'Artist X',
              status: 'pending',
              attempt_count: 0,
              created: 1704107200,
              updated: 1704107200,
              source_service: 'spotify',
              destination_service: 'youtube',
              source_playlist_name: 'Road Trip',
              destination_playlist_name: 'YT Road Trip',
            },
          ],
        });
      }),
    );

    renderWithProviders(<SyncQueuePage />);

    expect(screen.getByText('Sync Queue')).toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getByText('spotify → youtube')).toBeInTheDocument();
    });

    expect(screen.getByText('Road Trip')).toBeInTheDocument();
    expect(screen.getByText(/YT Road Trip/)).toBeInTheDocument();
    expect(screen.getByText('Neon Dreams')).toBeInTheDocument();
  });

  it('calls execute POST and shows success feedback', async () => {
    const user = userEvent.setup();
    let executeCalls = 0;

    server?.use(
      http.get('*/api/collections/sync_items/records', () => {
        return HttpResponse.json({
          page: 1,
          perPage: 50,
          totalItems: 1,
          totalPages: 1,
          items: [
            {
              id: 'sync-pending-1',
              mapping_id: 'mapping1',
              operation: 'add',
              service: 'youtube',
              track_title: 'Song',
              status: 'pending',
              attempt_count: 0,
              created: 1704107200,
              updated: 1704107200,
              source_service: 'spotify',
              destination_service: 'youtube',
            },
          ],
        });
      }),
      http.post('*/api/collections/sync_items/records/sync-pending-1/execute', () => {
        executeCalls += 1;
        return HttpResponse.json({
          id: 'sync-pending-1',
          mapping_id: 'mapping1',
          operation: 'add',
          service: 'youtube',
          status: 'done',
          attempt_count: 1,
          created: 1704107200,
          updated: 1704112000,
          source_service: 'spotify',
          destination_service: 'youtube',
        });
      }),
    );

    renderWithProviders(<SyncQueuePage />);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /execute/i })).toBeEnabled();
    });

    await user.click(screen.getByRole('button', { name: /execute/i }));

    await waitFor(() => {
      expect(executeCalls).toBe(1);
      expect(screen.getByText(/completed successfully/i)).toBeInTheDocument();
    });
  });

  it('shows View button and opens detail modal', async () => {
    const user = userEvent.setup()

    server?.use(
      http.get('*/api/collections/sync_items/records', () => {
        return HttpResponse.json({
          page: 1,
          perPage: 50,
          totalItems: 1,
          totalPages: 1,
          items: [
            {
              id: 'sync-view-1',
              mapping_id: 'mapping1',
              operation: 'add',
              service: 'youtube',
              track_title: 'Song',
              status: 'pending',
              attempt_count: 0,
              created: 1704107200,
              updated: 1704107200,
              source_service: 'spotify',
              destination_service: 'youtube',
            },
          ],
        })
      }),
      http.get('*/api/collections/sync_items/records/sync-view-1', () => {
        return HttpResponse.json({
          id: 'sync-view-1',
          mapping_id: 'mapping1',
          operation: 'add',
          service: 'youtube',
          track_title: 'Song',
          status: 'pending',
          attempt_count: 0,
          created: 1704107200,
          updated: 1704107200,
          source_service: 'spotify',
          destination_service: 'youtube',
        })
      }),
      http.get('*/api/collections/activity_logs/records', () => {
        return HttpResponse.json({ page: 1, perPage: 10, totalItems: 0, totalPages: 0, items: [] })
      }),
    )

    renderWithProviders(<SyncQueuePage />)

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /view/i })).toBeInTheDocument()
    })

    await user.click(screen.getByRole('button', { name: /view/i }))

    await waitFor(() => {
      expect(screen.getByRole('dialog')).toBeInTheDocument()
      expect(screen.getByText('Sync item details')).toBeInTheDocument()
    })
  })

  it('renders long track and playlist text without truncating it from the DOM', async () => {
    const longTitle =
      'This Is An Extremely Long Track Title That Should Remain Visible In The Sync Queue Table Cell'
    const longPlaylist =
      'My Very Long Source Playlist Name That Users Need To Read Fully Without Hover Tooltips'

    server?.use(
      http.get('*/api/collections/sync_items/records', () => {
        return HttpResponse.json({
          page: 1,
          perPage: 50,
          totalItems: 1,
          totalPages: 1,
          items: [
            {
              id: 'sync-long-1',
              mapping_id: 'mapping1',
              operation: 'add',
              service: 'youtube',
              track_title: longTitle,
              track_artist: 'Artist With A Long Name',
              status: 'pending',
              attempt_count: 0,
              created: 1704107200,
              updated: 1704107200,
              source_service: 'spotify',
              destination_service: 'youtube',
              source_playlist_name: longPlaylist,
              destination_playlist_name: 'Destination Playlist Also Quite Long For Testing',
            },
          ],
        })
      }),
    )

    renderWithProviders(<SyncQueuePage />)

    await waitFor(() => {
      expect(screen.getByText(longTitle)).toBeInTheDocument()
      expect(screen.getByText(longPlaylist)).toBeInTheDocument()
    })
  })

  it('shows analysis context in detail modal when present', async () => {
    const user = userEvent.setup()

    server?.use(
      http.get('*/api/collections/sync_items/records', () => {
        return HttpResponse.json({
          page: 1,
          perPage: 50,
          totalItems: 1,
          totalPages: 1,
          items: [
            {
              id: 'sync-context-1',
              mapping_id: 'mapping1',
              operation: 'add',
              service: 'youtube',
              track_title: 'Pelo Negro',
              status: 'pending',
              attempt_count: 0,
              created: 1704107200,
              updated: 1704107200,
              source_service: 'spotify',
              destination_service: 'youtube',
            },
          ],
        })
      }),
      http.get('*/api/collections/sync_items/records/sync-context-1', () => {
        return HttpResponse.json({
          id: 'sync-context-1',
          mapping_id: 'mapping1',
          operation: 'add',
          service: 'youtube',
          track_title: 'Pelo Negro',
          status: 'pending',
          attempt_count: 0,
          created: 1704107200,
          updated: 1704107200,
          source_service: 'spotify',
          destination_service: 'youtube',
          analysis_context_json: {
            version: 1,
            decision: { matched: false, score: 0.2, queued_add: true },
          },
        })
      }),
      http.get('*/api/collections/activity_logs/records', () => {
        return HttpResponse.json({ page: 1, perPage: 10, totalItems: 0, totalPages: 0, items: [] })
      }),
    )

    renderWithProviders(<SyncQueuePage />)

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /view/i })).toBeInTheDocument()
    })

    await user.click(screen.getByRole('button', { name: /view/i }))

    await waitFor(() => {
      expect(screen.getByText('Analysis context')).toBeInTheDocument()
      expect(screen.getByText(/queued_add/)).toBeInTheDocument()
    })
  })

  it('disables execute for done items', async () => {
    server?.use(
      http.get('*/api/collections/sync_items/records', () => {
        return HttpResponse.json({
          page: 1,
          perPage: 50,
          totalItems: 1,
          totalPages: 1,
          items: [
            {
              id: 'sync-done-1',
              mapping_id: 'mapping1',
              operation: 'rename',
              service: 'youtube',
              status: 'done',
              attempt_count: 1,
              created: 1704107200,
              updated: 1704110700,
              source_service: 'spotify',
              destination_service: 'youtube',
            },
          ],
        });
      }),
    );

    renderWithProviders(<SyncQueuePage />);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /execute/i })).toBeDisabled();
    });
  });
});
