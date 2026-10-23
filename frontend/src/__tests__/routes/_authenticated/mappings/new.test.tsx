import { describe, it, expect, vi } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import { Route } from '../../../../routes/_authenticated/mappings/new.lazy';
import { PlaylistLibrary } from '../../../../components/PlaylistLibrary';

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
  };
});

const NewMapping = Route.options.component as React.ComponentType;

function useApi() {
  const created: Record<string, unknown>[] = [];
  globalThis.mswServer?.use(
    http.get('*/api/auth/spotify/playlists', () =>
      HttpResponse.json([
        { id: 'sp-road', name: 'Road Trip', track_count: 12 },
        { id: 'sp-paired', name: 'Already Synced', track_count: 3 },
      ]),
    ),
    http.get('*/api/auth/youtube/playlists', () =>
      HttpResponse.json([
        { id: 'yt-road', name: 'Road Trip', itemCount: 10 },
        { id: 'yt-other', name: 'Other', itemCount: 4 },
      ]),
    ),
    http.get('*/api/collections/mappings/records', () =>
      HttpResponse.json({
        page: 1,
        perPage: 200,
        totalItems: 1,
        totalPages: 1,
        items: [{ id: 'm1', spotify_playlist_id: 'sp-paired', youtube_playlist_id: 'yt-x', spotify_playlist_name: 'Already Synced', youtube_playlist_name: 'X' }],
      }),
    ),
    http.get('*/api/backups', () => HttpResponse.json({ items: [] })),
    http.post('*/api/collections/mappings/records', async ({ request }) => {
      const body = (await request.json()) as Record<string, unknown>;
      created.push(body);
      return HttpResponse.json({ id: 'm2', ...body }, { status: 201 });
    }),
  );
  return created;
}

const newClient = () => new QueryClient({ defaultOptions: { queries: { retry: false } } });

describe('New mapping page', () => {
  it('renders after the playlist library has filled the shared cache', async () => {
    useApi();
    const client = newClient();
    const { unmount } = render(
      <QueryClientProvider client={client}>
        <PlaylistLibrary service="spotify" />
        <PlaylistLibrary service="youtube" />
      </QueryClientProvider>,
    );
    await screen.findAllByText('Road Trip');
    await waitFor(() => expect(client.getQueryData(['mappings', 'all'])).toBeDefined());
    unmount();

    render(
      <QueryClientProvider client={client}>
        <NewMapping />
      </QueryClientProvider>,
    );
    const spotifyList = screen.getByRole('radiogroup', { name: 'spotify playlists' });
    expect(within(spotifyList).getByRole('radio', { name: /Already Synced/ })).toBeDisabled();
    expect(within(spotifyList).getByRole('radio', { name: /Road Trip/ })).toBeEnabled();
  });

  it('pairs two existing playlists', async () => {
    const created = useApi();
    const user = userEvent.setup();
    render(
      <QueryClientProvider client={newClient()}>
        <NewMapping />
      </QueryClientProvider>,
    );
    const spotifyList = await screen.findByRole('radiogroup', { name: 'spotify playlists' });
    await user.click(await within(spotifyList).findByRole('radio', { name: /Road Trip/ }));
    const youtubeList = screen.getByRole('radiogroup', { name: 'youtube playlists' });
    const sameName = await within(youtubeList).findByRole('radio', { name: /Road Trip.*Same name/ });
    await user.click(sameName);
    await user.click(screen.getByRole('button', { name: 'Pair playlists' }));

    await waitFor(() => expect(created).toHaveLength(1));
    expect(created[0]).toMatchObject({
      spotify_playlist_id: 'sp-road',
      youtube_playlist_id: 'yt-road',
      spotify_playlist_name: 'Road Trip',
      youtube_playlist_name: 'Road Trip',
    });
    expect(created[0].create_playlist).toBeUndefined();
    expect(navigate).toHaveBeenCalledWith({ to: '/mappings' });
  });

  it('creates the YouTube side from a Spotify playlist', async () => {
    const created = useApi();
    const user = userEvent.setup();
    render(
      <QueryClientProvider client={newClient()}>
        <NewMapping />
      </QueryClientProvider>,
    );
    const spotifyList = await screen.findByRole('radiogroup', { name: 'spotify playlists' });
    await user.click(await within(spotifyList).findByRole('radio', { name: /Road Trip/ }));
    const youtubeList = screen.getByRole('radiogroup', { name: 'youtube playlists' });
    await user.click(within(youtubeList).getByRole('radio', { name: /New YouTube playlist/ }));

    // Only one side can be new.
    expect(within(spotifyList).getByRole('radio', { name: /New Spotify playlist/ })).toBeDisabled();
    expect(screen.getByText(/uses 50 quota units/)).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'New YouTube playlist name' })).toHaveAttribute('placeholder', 'Road Trip');

    await user.click(screen.getByRole('button', { name: 'Create YouTube playlist and pair' }));
    await waitFor(() => expect(created).toHaveLength(1));
    expect(created[0]).toMatchObject({
      spotify_playlist_id: 'sp-road',
      youtube_playlist_id: '',
      youtube_playlist_name: 'Road Trip',
      create_playlist: 'youtube',
    });
  });

  it('creates the Spotify side with a custom name', async () => {
    const created = useApi();
    const user = userEvent.setup();
    render(
      <QueryClientProvider client={newClient()}>
        <NewMapping />
      </QueryClientProvider>,
    );
    const spotifyList = await screen.findByRole('radiogroup', { name: 'spotify playlists' });
    await user.click(within(spotifyList).getByRole('radio', { name: /New Spotify playlist/ }));
    const submit = screen.getByRole('button', { name: 'Create Spotify playlist and pair' });
    expect(submit).toBeDisabled(); // nothing picked on YouTube yet

    const youtubeList = screen.getByRole('radiogroup', { name: 'youtube playlists' });
    await user.click(await within(youtubeList).findByRole('radio', { name: /Other/ }));
    await user.type(screen.getByRole('textbox', { name: 'New Spotify playlist name' }), 'Other (from YouTube)');
    await user.click(submit);

    await waitFor(() => expect(created).toHaveLength(1));
    expect(created[0]).toMatchObject({
      spotify_playlist_id: '',
      youtube_playlist_id: 'yt-other',
      spotify_playlist_name: 'Other (from YouTube)',
      create_playlist: 'spotify',
    });
  });
});
