import { describe, it, expect, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { Route } from '../../../../../routes/_authenticated/mappings/$mappingId/blacklist.lazy';
import { http, HttpResponse } from 'msw';

vi.mock('@tanstack/react-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@tanstack/react-router')>();
  return {
    ...actual,
    Link: ({ to, children, className }: { to: string; children: React.ReactNode; className?: string }) => (
      <a href={to} className={className}>
        {children}
      </a>
    ),
    useParams: () => ({ mappingId: 'mapping1' }),
  };
});

const BlacklistPage = Route.options.component as React.ComponentType;

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <BlacklistPage />
    </QueryClientProvider>,
  );
}

describe('Blocked tracks page', () => {
  const server = globalThis.mswServer;

  it('lists blocked tracks with their reason, falling back to the track id', async () => {
    renderPage();

    expect(await screen.findByText('spotify_track_456')).toBeInTheDocument();
    expect(screen.getByText('youtube_video_789')).toBeInTheDocument();
    expect(screen.getByText(/not found/)).toBeInTheDocument();
  });

  it('shows the track title when the API provides it', async () => {
    server?.use(
      http.get('*/api/collections/blacklist/records', () =>
        HttpResponse.json({
          page: 1,
          perPage: 100,
          totalItems: 1,
          totalPages: 1,
          items: [
            {
              id: 'bl1',
              mapping_id: 'mapping1',
              service: 'youtube',
              track_id: 'sp1',
              track_title: 'Umbrella',
              track_artist: 'Moses Bliss',
              reason: 'Blocked from the Sync Queue',
              skip_counter: 0,
              last_skipped_at: null,
              created: 1704107200,
              updated: 1704107200,
            },
          ],
        }),
      ),
    );
    renderPage();

    expect(await screen.findByText('Umbrella')).toBeInTheDocument();
    expect(screen.getByText(/Moses Bliss · Blocked from the Sync Queue/)).toBeInTheDocument();
  });

  it('shows the mapping in the description', async () => {
    renderPage();

    await waitFor(() => {
      expect(screen.getByText(/My Spotify Playlist ⇄ My YouTube Playlist/)).toBeInTheDocument();
    });
  });

  it('shows an empty state', async () => {
    server?.use(
      http.get('*/api/collections/blacklist/records', () =>
        HttpResponse.json({ page: 1, perPage: 100, totalItems: 0, totalPages: 0, items: [] }),
      ),
    );
    renderPage();

    expect(await screen.findByText(/No blocked tracks/)).toBeInTheDocument();
  });

  it('unblocks a track', async () => {
    const user = userEvent.setup();
    let deleted = '';
    server?.use(
      http.delete('*/api/collections/blacklist/records/:id', ({ params }) => {
        deleted = String(params.id);
        return new HttpResponse(null, { status: 204 });
      }),
    );
    renderPage();

    const buttons = await screen.findAllByRole('button', { name: 'Unblock' });
    await user.click(buttons[0]);

    await waitFor(() => expect(deleted).not.toBe(''));
  });

  it('shows an error when loading fails', async () => {
    server?.use(http.get('*/api/collections/blacklist/records', () => new HttpResponse(null, { status: 500 })));
    renderPage();

    expect(await screen.findByText(/Error loading blocked tracks/)).toBeInTheDocument();
  });

  it('links back to the mapping', async () => {
    renderPage();

    expect(await screen.findByText('Back to mapping')).toHaveAttribute('href', '/mappings/$mappingId/edit');
  });
});
