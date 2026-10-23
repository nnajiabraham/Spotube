import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import { Route } from '../../../routes/_authenticated/review.lazy';

const navigate = vi.fn();

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
    useSearch: () => ({}),
  };
});

const ReviewPage = (Route.options.component ?? (() => null)) as React.ComponentType;

const renderPage = () => {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <ReviewPage />
    </QueryClientProvider>,
  );
};

const reviewItem = {
  id: 'r1',
  mapping_id: 'mapping1',
  operation: 'add',
  service: 'youtube',
  track_id: 'sp-fearless',
  track_title: 'Fearless',
  track_artist: 'Sounds Of Salem',
  status: 'review',
  error_message: 'best match scored 0.62',
  attempt_count: 1,
  created: 1704107200,
  updated: 1704107200,
  source_service: 'spotify',
  destination_service: 'youtube',
  source_playlist_name: 'Worship',
  destination_playlist_name: 'YT Worship',
  last_run: {
    source: { platform: 'spotify', id: 'sp-fearless', title: 'Fearless', artist: 'Sounds Of Salem', duration_ms: 300000 },
    search_query: 'Sounds Of Salem Fearless',
    outcome: 'no_match',
    candidates: [
      {
        rank: 1,
        id: 'vidLive0001',
        title: 'Fearless (Live)',
        artist: 'Sounds Of Salem',
        score: 0.62,
        breakdown: { total: 0.62, title: 0.8, artist: 1, artist_known: true, adjustments: [{ reason: 'only one is live', delta: -0.2 }] },
      },
    ],
  },
};

type Captured = { resolve: unknown[]; searches: string[] };

function useReviewHandlers(captured: Captured) {
  globalThis.mswServer?.use(
    http.get('*/api/collections/mappings/records', () =>
      HttpResponse.json({ page: 1, perPage: 100, totalItems: 0, totalPages: 1, items: [] }),
    ),
    http.get('*/api/collections/sync_items/records', ({ request }) => {
      const url = new URL(request.url);
      expect(url.searchParams.get('status')).toBe('review');
      expect(url.searchParams.get('include')).toBe('last_run');
      return HttpResponse.json({ page: 1, perPage: 10, totalItems: 1, totalPages: 1, items: [reviewItem] });
    }),
    http.post('*/api/collections/sync_items/records/:id/resolve', async ({ request }) => {
      const body = (await request.json()) as { action: string };
      captured.resolve.push(body);
      return HttpResponse.json({
        ...reviewItem,
        status: body.action === 'unavailable' ? 'skipped' : 'done',
        error_message: body.action === 'unavailable' ? 'not on YouTube' : '',
      });
    }),
    http.get('*/api/tracks/search', ({ request }) => {
      const url = new URL(request.url);
      captured.searches.push(url.searchParams.get('q') ?? '');
      expect(url.searchParams.get('service')).toBe('youtube');
      expect(url.searchParams.get('item_id')).toBe('r1');
      return HttpResponse.json({
        cached: false,
        quota_cost: 101,
        items: [{ id: 'vidStudio01', title: 'Fearless', artist: 'Sounds Of Salem', channel: 'Sounds Of Salem - Topic', duration_ms: 301000, score: 0.98 }],
      });
    }),
    http.get('*/api/tracks/lookup', ({ request }) => {
      const url = new URL(request.url);
      expect(url.searchParams.get('ref')).toBe('https://youtu.be/vidPasted01');
      return HttpResponse.json({ id: 'vidPasted01', title: 'Fearless', artist: 'Sounds Of Salem', score: 0.95 });
    }),
  );
}

describe('Review page', () => {
  let captured: Captured;

  beforeEach(() => {
    vi.clearAllMocks();
    captured = { resolve: [], searches: [] };
    useReviewHandlers(captured);
  });

  it('shows the source track, the closest candidate and why its score fell short', async () => {
    const user = userEvent.setup();
    renderPage();

    expect(await screen.findByText('best match scored 0.62')).toBeInTheDocument();
    expect(screen.getByText('Fearless (Live)')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '62%' }));
    expect(screen.getByRole('tooltip')).toHaveTextContent('only one is live');
  });

  it('uses the suggested candidate in one click', async () => {
    const user = userEvent.setup();
    renderPage();

    await user.click(await screen.findByRole('button', { name: 'Use this' }));
    await waitFor(() => expect(captured.resolve).toEqual([{ action: 'match', target_id: 'vidLive0001' }]));
    expect(await screen.findByRole('status')).toHaveTextContent('Added “Fearless” to YT Worship');
  });

  it('marks a song as not on the destination without searching', async () => {
    const user = userEvent.setup();
    renderPage();

    await user.click(await screen.findByRole('button', { name: /not on youtube/i }));
    await waitFor(() => expect(captured.resolve).toEqual([{ action: 'unavailable' }]));
    expect(await screen.findByRole('status')).toHaveTextContent("won't be searched again");
    expect(captured.searches).toEqual([]);
  });

  it('searches YouTube only when submitted, then picks a result', async () => {
    const user = userEvent.setup();
    renderPage();

    await user.click(await screen.findByRole('button', { name: /find on youtube/i }));
    const dialog = await screen.findByRole('dialog');
    await user.click(within(dialog).getByRole('tab', { name: /search youtube/i }));
    const input = within(dialog).getByRole('textbox', { name: /search youtube/i });
    expect(input).toHaveValue('Sounds Of Salem Fearless');
    await user.type(input, ' studio');
    expect(captured.searches).toEqual([]);

    await user.click(within(dialog).getByRole('button', { name: 'Search' }));
    expect(await within(dialog).findByText('98%')).toBeInTheDocument();
    expect(captured.searches).toEqual(['Sounds Of Salem Fearless studio']);
    expect(within(dialog).getByText(/uses 101 of the 10,000 daily quota units/i)).toBeInTheDocument();

    await user.click(within(dialog).getByRole('button', { name: 'Use this' }));
    await waitFor(() => expect(captured.resolve).toEqual([{ action: 'match', target_id: 'vidStudio01' }]));
  });

  it('resolves a pasted link', async () => {
    const user = userEvent.setup();
    renderPage();

    await user.click(await screen.findByRole('button', { name: /find on youtube/i }));
    const dialog = await screen.findByRole('dialog');
    await user.click(within(dialog).getByRole('tab', { name: /paste a link/i }));
    await user.type(within(dialog).getByRole('textbox', { name: /youtube link or id/i }), 'https://youtu.be/vidPasted01');
    await user.click(within(dialog).getByRole('button', { name: 'Look up' }));
    await user.click(await within(dialog).findByRole('button', { name: 'Use this' }));
    await waitFor(() => expect(captured.resolve).toEqual([{ action: 'match', target_id: 'vidPasted01' }]));
  });
});
