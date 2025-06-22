import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import { Route } from '../../../routes/_authenticated/logs.lazy';

// Extract the component from the Route
const ActivityLogsPage = Route.options.component as React.ComponentType;

const renderWithProviders = (ui: React.ReactElement) => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });
  
  return render(
    <QueryClientProvider client={queryClient}>
      {ui}
    </QueryClientProvider>
  );
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
      sync_item_id: '',
      job_type: 'analysis' as const,
      created: '2024-01-01T12:00:00Z',
      updated: '2024-01-01T12:00:00Z',
    },
    {
      id: 'log2',
      level: 'warn' as const,
      message: 'Rate limit encountered, retrying in 30 seconds',
      sync_item_id: 'sync_item_1',
      job_type: 'execution' as const,
      created: '2024-01-01T12:05:00Z',
      updated: '2024-01-01T12:05:00Z',
    },
    {
      id: 'log3',
      level: 'error' as const,
      message: 'Failed to add track: Track not found',
      sync_item_id: 'sync_item_2',
      job_type: 'execution' as const,
      created: '2024-01-01T12:10:00Z',
      updated: '2024-01-01T12:10:00Z',
    },
  ],
};

const mockSyncItem = {
  id: 'sync_item_1',
  mapping_id: 'mapping1',
  service: 'spotify',
  action: 'add_track',
  status: 'running',
  source_track_id: 'youtube_video_456',
  source_track_title: 'Test Song',
  source_service: 'youtube',
  destination_service: 'spotify',
  payload: '',
  attempts: 2,
  last_error: 'Rate limit exceeded',
  created: '2024-01-01T11:30:00Z',
  updated: '2024-01-01T12:10:00Z',
};

describe('Activity Logs Page', () => {
  const server = globalThis.mswServer;

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders activity logs table with data', async () => {
    server?.use(
      http.get('*/api/collections/activity_logs/records', () => {
        return HttpResponse.json(mockActivityLogs);
      })
    );

    renderWithProviders(<ActivityLogsPage />);

    // Check page title and description
    expect(screen.getByText('Activity Logs')).toBeInTheDocument();
    expect(screen.getByText('Monitor system activity and sync job execution')).toBeInTheDocument();

    // Wait for data to load and check table content
    await waitFor(() => {
      expect(screen.getByText('Starting sync analysis job')).toBeInTheDocument();
    });

    expect(screen.getByText('Rate limit encountered, retrying in 30 seconds')).toBeInTheDocument();
    expect(screen.getByText('Failed to add track: Track not found')).toBeInTheDocument();

    // Check that levels are displayed with badges
    expect(screen.getByText('INFO')).toBeInTheDocument();
    expect(screen.getByText('WARN')).toBeInTheDocument();
    expect(screen.getByText('ERROR')).toBeInTheDocument();

    // Check that job types are displayed
    expect(screen.getByText('ANALYSIS')).toBeInTheDocument();
    expect(screen.getAllByText('EXECUTION')).toHaveLength(2);
  });

  it('filters logs by level correctly', async () => {
    const user = userEvent.setup();

    server?.use(
      http.get('*/api/collections/activity_logs/records', ({ request }) => {
        const url = new URL(request.url);
        const filter = url.searchParams.get('filter');
        
        if (filter?.includes('level = "error"')) {
          return HttpResponse.json({
            ...mockActivityLogs,
            totalItems: 1,
            items: [mockActivityLogs.items[2]], // Only error log
          });
        }
        
        return HttpResponse.json(mockActivityLogs);
      })
    );

    renderWithProviders(<ActivityLogsPage />);

    // Wait for initial data
    await waitFor(() => {
      expect(screen.getByText('Starting sync analysis job')).toBeInTheDocument();
    });

    // Filter by error level
    const levelFilter = screen.getByDisplayValue('All Levels');
    await user.selectOptions(levelFilter, 'error');

    // Wait for filtered results
    await waitFor(() => {
      expect(screen.queryByText('Starting sync analysis job')).not.toBeInTheDocument();
      expect(screen.queryByText('Rate limit encountered, retrying in 30 seconds')).not.toBeInTheDocument();
      expect(screen.getByText('Failed to add track: Track not found')).toBeInTheDocument();
    });
  });

  it('filters logs by job type correctly', async () => {
    const user = userEvent.setup();

    server?.use(
      http.get('*/api/collections/activity_logs/records', ({ request }) => {
        const url = new URL(request.url);
        const filter = url.searchParams.get('filter');
        
        if (filter?.includes('job_type = "analysis"')) {
          return HttpResponse.json({
            ...mockActivityLogs,
            totalItems: 1,
            items: [mockActivityLogs.items[0]], // Only analysis log
          });
        }
        
        return HttpResponse.json(mockActivityLogs);
      })
    );

    renderWithProviders(<ActivityLogsPage />);

    // Wait for initial data
    await waitFor(() => {
      expect(screen.getByText('Starting sync analysis job')).toBeInTheDocument();
    });

    // Filter by analysis job type
    const jobTypeFilter = screen.getByDisplayValue('All Job Types');
    await user.selectOptions(jobTypeFilter, 'analysis');

    // Wait for filtered results
    await waitFor(() => {
      expect(screen.getByText('Starting sync analysis job')).toBeInTheDocument();
      expect(screen.queryByText('Rate limit encountered, retrying in 30 seconds')).not.toBeInTheDocument();
      expect(screen.queryByText('Failed to add track: Track not found')).not.toBeInTheDocument();
    });
  });

  it('opens sync item modal when clicking on log with sync_item_id', async () => {
    const user = userEvent.setup();

    server?.use(
      http.get('*/api/collections/activity_logs/records', () => {
        return HttpResponse.json(mockActivityLogs);
      }),
      http.get('*/api/collections/sync_items/records/sync_item_1', () => {
        return HttpResponse.json(mockSyncItem);
      })
    );

    renderWithProviders(<Route.options.component />);

    // Wait for data to load
    await waitFor(() => {
      expect(screen.getByText('Rate limit encountered, retrying in 30 seconds')).toBeInTheDocument();
    });

    // Click on the log message with sync_item_id
    const logWithSyncItem = screen.getByText('Rate limit encountered, retrying in 30 seconds');
    await user.click(logWithSyncItem);

    // Wait for modal to appear
    await waitFor(() => {
      expect(screen.getByText('Sync Item Details')).toBeInTheDocument();
    });

    // Check modal content
    expect(screen.getByText('Test Song')).toBeInTheDocument();
    expect(screen.getByText('running')).toBeInTheDocument();
    expect(screen.getByText('Rate limit exceeded')).toBeInTheDocument();
    expect(screen.getByText('youtube → spotify')).toBeInTheDocument();
  });

  it('closes sync item modal when clicking close button', async () => {
    const user = userEvent.setup();

    server?.use(
      http.get('*/api/collections/activity_logs/records', () => {
        return HttpResponse.json(mockActivityLogs);
      }),
      http.get('*/api/collections/sync_items/records/sync_item_1', () => {
        return HttpResponse.json(mockSyncItem);
      })
    );

    renderWithProviders(<Route.options.component />);

    // Wait for data and open modal
    await waitFor(() => {
      expect(screen.getByText('Rate limit encountered, retrying in 30 seconds')).toBeInTheDocument();
    });

    const logWithSyncItem = screen.getByText('Rate limit encountered, retrying in 30 seconds');
    await user.click(logWithSyncItem);

    await waitFor(() => {
      expect(screen.getByText('Sync Item Details')).toBeInTheDocument();
    });

    // Close modal
    const closeButton = screen.getByRole('button', { name: 'Close modal' });
    await user.click(closeButton);

    // Modal should be gone
    await waitFor(() => {
      expect(screen.queryByText('Sync Item Details')).not.toBeInTheDocument();
    });
  });

  it('shows loading state while fetching data', () => {
    server?.use(
      http.get('*/api/collections/activity_logs/records', async () => {
        await new Promise(resolve => setTimeout(resolve, 100));
        return HttpResponse.json(mockActivityLogs);
      })
    );

    renderWithProviders(<Route.options.component />);

    // Check for loading state (shimmer effects)
    const loadingElements = screen.getAllByRole('status', { hidden: true });
    expect(loadingElements.length).toBeGreaterThan(0);
  });

  it('shows error state when API fails', async () => {
    server?.use(
      http.get('*/api/collections/activity_logs/records', () => {
        return HttpResponse.json(
          { error: 'Internal server error' },
          { status: 500 }
        );
      })
    );

    renderWithProviders(<Route.options.component />);

    await waitFor(() => {
      expect(screen.getByText('Error loading activity logs')).toBeInTheDocument();
    });

    expect(screen.getByText('Unable to fetch activity logs. Please try refreshing.')).toBeInTheDocument();
  });

  it('shows empty state when no logs are found', async () => {
    server?.use(
      http.get('*/api/collections/activity_logs/records', () => {
        return HttpResponse.json({
          page: 1,
          perPage: 50,
          totalItems: 0,
          totalPages: 0,
          items: [],
        });
      })
    );

    renderWithProviders(<Route.options.component />);

    await waitFor(() => {
      expect(screen.getByText('No activity logs found')).toBeInTheDocument();
    });

    expect(screen.getByText('Activity logs will appear here when system operations occur.')).toBeInTheDocument();
  });

  it('refreshes data when refresh button is clicked', async () => {
    const user = userEvent.setup();
    let callCount = 0;

    server?.use(
      http.get('*/api/collections/activity_logs/records', () => {
        callCount++;
        return HttpResponse.json(mockActivityLogs);
      })
    );

    renderWithProviders(<Route.options.component />);

    // Wait for initial load
    await waitFor(() => {
      expect(screen.getByText('Starting sync analysis job')).toBeInTheDocument();
    });

    expect(callCount).toBe(1);

    // Click refresh button
    const refreshButton = screen.getByText('Refresh');
    await user.click(refreshButton);

    // Should trigger another API call
    await waitFor(() => {
      expect(callCount).toBe(2);
    });
  });
}); 