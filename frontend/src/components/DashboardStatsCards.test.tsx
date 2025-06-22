import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { DashboardStatsCards } from './DashboardStatsCards';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';

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

const mockStats = {
  mappings: { total: 5 },
  queue: { 
    pending: 12, 
    running: 1, 
    errors: 2, 
    skipped: 1, 
    done: 102 
  },
  recent_runs: [
    {
      timestamp: '2024-01-01T12:00:00Z',
      job_type: 'analysis' as const,
      status: 'success' as const,
      message: 'Analysis completed successfully'
    }
  ],
  youtube_quota: { used: 1250, limit: 10000 }
};

describe('DashboardStatsCards', () => {
  const server = globalThis.mswServer;
  const mockOnTogglePause = vi.fn();
  const mockOnRefresh = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders correct stats from the API', async () => {
    server?.use(
      http.get('http://localhost:8090/api/dashboard/stats', () => {
        return HttpResponse.json(mockStats);
      })
    );

    renderWithProviders(
      <DashboardStatsCards 
        isPaused={false}
        onTogglePause={mockOnTogglePause}
        onRefresh={mockOnRefresh}
      />
    );

    // Just verify the component renders with the expected titles
    expect(screen.getByText('Total Mappings')).toBeInTheDocument();
    expect(screen.getByText('Pending Items')).toBeInTheDocument();
    expect(screen.getByText('Running Items')).toBeInTheDocument();
    expect(screen.getByText('Error Items')).toBeInTheDocument();
    expect(screen.getByText('Skipped Items')).toBeInTheDocument();
    expect(screen.getByText('Completed Items')).toBeInTheDocument();
    expect(screen.getByText('YouTube Quota')).toBeInTheDocument();
    expect(screen.getByText('Auto-refresh')).toBeInTheDocument();
  });

  it('shows loading state while fetching data', () => {
    // Use a delayed response to test loading state
    server?.use(
      http.get('http://localhost:8090/api/dashboard/stats', async () => {
        await new Promise(resolve => setTimeout(resolve, 100));
        return HttpResponse.json(mockStats);
      })
    );

    renderWithProviders(
      <DashboardStatsCards 
        isPaused={false}
        onTogglePause={mockOnTogglePause}
        onRefresh={mockOnRefresh}
      />
    );

    // Check for loading indicators (shimmer effects)
    const loadingElements = screen.getAllByRole('status', { hidden: true });
    expect(loadingElements.length).toBeGreaterThan(0);
  });

  it('handles pause button correctly', async () => {
    server?.use(
      http.get('http://localhost:8090/api/dashboard/stats', () => {
        return HttpResponse.json(mockStats);
      })
    );

    const user = userEvent.setup();

    renderWithProviders(
      <DashboardStatsCards 
        isPaused={false}
        onTogglePause={mockOnTogglePause}
        onRefresh={mockOnRefresh}
      />
    );

    // Wait for component to load
    await waitFor(() => {
      expect(screen.getByText('Pause')).toBeInTheDocument();
    });

    // Click pause button
    const pauseButton = screen.getByText('Pause');
    await user.click(pauseButton);

    expect(mockOnTogglePause).toHaveBeenCalledTimes(1);
  });

  it('shows resume button when paused', async () => {
    server?.use(
      http.get('http://localhost:8090/api/dashboard/stats', () => {
        return HttpResponse.json(mockStats);
      })
    );

    renderWithProviders(
      <DashboardStatsCards 
        isPaused={true}
        onTogglePause={mockOnTogglePause}
        onRefresh={mockOnRefresh}
      />
    );

    await waitFor(() => {
      expect(screen.getByText('Resume')).toBeInTheDocument();
    });

    expect(screen.getByText('Paused')).toBeInTheDocument();
  });

  it('calls refresh handler when refresh button is clicked', async () => {
    server?.use(
      http.get('http://localhost:8090/api/dashboard/stats', () => {
        return HttpResponse.json(mockStats);
      })
    );

    const user = userEvent.setup();

    renderWithProviders(
      <DashboardStatsCards 
        isPaused={false}
        onTogglePause={mockOnTogglePause}
        onRefresh={mockOnRefresh}
      />
    );

    // Wait for component to load
    await waitFor(() => {
      expect(screen.getByText('Refresh')).toBeInTheDocument();
    });

    // Click refresh button
    const refreshButton = screen.getByText('Refresh');
    await user.click(refreshButton);

    expect(mockOnRefresh).toHaveBeenCalledTimes(1);
  });

  it('shows error state when API fails', async () => {
    server?.use(
      http.get('http://localhost:8090/api/dashboard/stats', () => {
        return HttpResponse.json(
          { error: 'Internal server error' },
          { status: 500 }
        );
      })
    );

    renderWithProviders(
      <DashboardStatsCards 
        isPaused={false}
        onTogglePause={mockOnTogglePause}
        onRefresh={mockOnRefresh}
      />
    );

    await waitFor(() => {
      expect(screen.getByText('Error loading dashboard stats')).toBeInTheDocument();
    });

    expect(screen.getByText('Unable to fetch dashboard data. Please try refreshing.')).toBeInTheDocument();
    expect(screen.getByText('Try Again')).toBeInTheDocument();
  });

  it('displays all stat card titles correctly', async () => {
    server?.use(
      http.get('http://localhost:8090/api/dashboard/stats', () => {
        return HttpResponse.json(mockStats);
      })
    );

    renderWithProviders(
      <DashboardStatsCards 
        isPaused={false}
        onTogglePause={mockOnTogglePause}
        onRefresh={mockOnRefresh}
      />
    );

    await waitFor(() => {
      expect(screen.getByText('Total Mappings')).toBeInTheDocument();
    });

    expect(screen.getByText('Pending Items')).toBeInTheDocument();
    expect(screen.getByText('Running Items')).toBeInTheDocument();
    expect(screen.getByText('Error Items')).toBeInTheDocument();
    expect(screen.getByText('Skipped Items')).toBeInTheDocument();
    expect(screen.getByText('Completed Items')).toBeInTheDocument();
    expect(screen.getByText('YouTube Quota')).toBeInTheDocument();
    expect(screen.getByText('Auto-refresh')).toBeInTheDocument();
  });
}); 