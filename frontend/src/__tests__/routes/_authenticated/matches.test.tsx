import { describe, it, expect, vi } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import { Route } from '../../../routes/_authenticated/matches.lazy';

vi.mock('@tanstack/react-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@tanstack/react-router')>();
  return { ...actual };
});

const MatchesPage = (Route.options.component ?? (() => null)) as React.ComponentType;

const links = [
  { id: 'l1', spotify_id: 'sp1', youtube_id: 'yt1', title: 'Umbrella', artist: 'Moses Bliss', origin: 'auto', score: 0.97, created: 1, updated: 1 },
  { id: 'l2', spotify_id: 'sp2', title: 'Rare Song', artist: 'Someone', origin: 'manual', created: 1, updated: 1 },
];

describe('Matches page', () => {
  it('lists pairs and unavailable songs, filters, and forgets a match', async () => {
    const requests: string[] = [];
    const deleted: string[] = [];
    globalThis.mswServer?.use(
      http.get('*/api/track-links', ({ request }) => {
        const url = new URL(request.url);
        requests.push(url.searchParams.get('filter') ?? '');
        const filter = url.searchParams.get('filter');
        const items = filter === 'missing_youtube' ? links.filter((l) => !l.youtube_id) : links;
        return HttpResponse.json({ page: 1, perPage: 50, totalItems: items.length, totalPages: 1, items });
      }),
      http.delete('*/api/track-links/:id', ({ params }) => {
        deleted.push(String(params.id));
        return new HttpResponse(null, { status: 204 });
      }),
    );
    const user = userEvent.setup();
    render(
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <MatchesPage />
      </QueryClientProvider>,
    );

    const umbrella = (await screen.findByText('Umbrella')).closest('tr')!;
    expect(within(umbrella).getByText('97%')).toBeInTheDocument();
    expect(within(umbrella).getByRole('link', { name: /yt1/ })).toHaveAttribute('href', 'https://www.youtube.com/watch?v=yt1');
    const rare = screen.getByText('Rare Song').closest('tr')!;
    expect(within(rare).getByText('Not on YouTube')).toBeInTheDocument();
    expect(within(rare).getByText('You decided')).toBeInTheDocument();

    await user.click(screen.getByRole('tab', { name: 'Not on YouTube' }));
    await waitFor(() => expect(requests).toContain('missing_youtube'));
    await waitFor(() => expect(screen.queryByText('Umbrella')).not.toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: /forget/i }));
    await waitFor(() => expect(deleted).toEqual(['l2']));
  });
});
