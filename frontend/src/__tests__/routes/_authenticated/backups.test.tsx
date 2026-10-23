import { describe, it, expect, vi } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import { Route as ListRoute } from '../../../routes/_authenticated/backups/index.lazy';
import { Route as DetailRoute } from '../../../routes/_authenticated/backups/$backupId.lazy';

vi.mock('@tanstack/react-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@tanstack/react-router')>();
  return {
    ...actual,
    Link: ({ to, children, className }: { to: string; children: React.ReactNode; className?: string }) => (
      <a href={to} className={className}>
        {children}
      </a>
    ),
    useNavigate: () => vi.fn(),
  };
});

const BackupsPage = (ListRoute.options.component ?? (() => null)) as React.ComponentType;
const BackupDetail = (DetailRoute.options.component ?? (() => null)) as React.ComponentType;

const renderPage = (Page: React.ComponentType) =>
  render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <Page />
    </QueryClientProvider>,
  );

const summary = {
  id: 's1',
  backup_id: 'b1',
  playlist_name: 'Road Trip',
  track_count: 2,
  source: 'initial',
  created: 1792774000,
  confirmed_at: 1792774300,
};
const backup = {
  id: 'b1',
  service: 'spotify',
  playlist_id: 'sp-road',
  playlist_name: 'Road Trip',
  enabled: true,
  interval_hours: 24,
  keep_snapshots: 30,
  last_checked_at: 1792774300,
  created: 1792774000,
  updated: 1792774300,
  snapshot_count: 1,
  latest_snapshot: summary,
};
const tracks = [
  { position: 0, id: 'sp1', title: 'Umbrella', artist: 'Moses Bliss', duration_ms: 189000, spotify_id: 'sp1', youtube_id: 'yt1' },
  { position: 1, id: 'sp2', title: 'Rare Song', artist: 'Someone', duration_ms: 200000, spotify_id: 'sp2' },
];

describe('Backups page', () => {
  it('lists backups, backs one up now, and adds a playlist from the library', async () => {
    const created: unknown[] = [];
    let items = [backup];
    globalThis.mswServer?.use(
      http.get('*/api/backups', () => HttpResponse.json({ items })),
      http.post('*/api/backups/:id/run', () => HttpResponse.json({ snapshot: summary, changed: false })),
      http.post('*/api/backups', async ({ request }) => {
        const body = (await request.json()) as Record<string, string>;
        created.push(body);
        const b = { ...backup, id: 'b2', playlist_id: body.playlist_id, playlist_name: body.playlist_name };
        items = [...items, b];
        return HttpResponse.json(b, { status: 201 });
      }),
      http.get('*/api/auth/spotify/playlists', () =>
        HttpResponse.json([
          { id: 'playlist1', name: 'My Awesome Playlist', track_count: 4 },
          { id: 'playlist2', name: 'Chill Vibes', track_count: 9 },
        ]),
      ),
      http.get('*/api/collections/mappings/records', () =>
        HttpResponse.json({ page: 1, perPage: 100, totalItems: 0, totalPages: 0, items: [] }),
      ),
    );
    const user = userEvent.setup();
    renderPage(BackupsPage);

    const row = (await screen.findByText('Road Trip')).closest('li')!;
    expect(within(row).getByText('2 tracks')).toBeInTheDocument();
    expect(within(row).getByText('first')).toBeInTheDocument();
    await user.click(within(row).getByRole('button', { name: /back up now/i }));
    expect(await within(row).findByText('No changes since the last snapshot.')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: /add playlist/i }));
    const dialog = await screen.findByRole('dialog');
    const chill = (await within(dialog).findByText('Chill Vibes')).closest('li')!;
    await user.click(within(chill).getByRole('button', { name: 'Back up' }));
    await waitFor(() => expect(created).toEqual([{ service: 'spotify', playlist_id: 'playlist2', playlist_name: 'Chill Vibes' }]));
    expect(await within(chill).findByText(/Backed up · 1/)).toBeInTheDocument();
  });
});

describe('Backup detail page', () => {
  it('shows the snapshot with IDs on both services and restores missing tracks', async () => {
    vi.spyOn(DetailRoute, 'useParams').mockReturnValue({ backupId: 'b1' } as never);
    const restores: unknown[] = [];
    globalThis.mswServer?.use(
      http.get('*/api/backups/b1', () => HttpResponse.json(backup)),
      http.get('*/api/backups/b1/snapshots', () => HttpResponse.json({ items: [summary] })),
      http.get('*/api/backups/snapshots/s1', () =>
        HttpResponse.json({ ...summary, tracks, added_tracks: [], removed_tracks: [], has_previous: false }),
      ),
      http.post('*/api/backups/snapshots/s1/restore', async ({ request }) => {
        restores.push(await request.json());
        return HttpResponse.json({
          mode: 'missing',
          service: 'spotify',
          playlist_id: 'sp-road',
          playlist_name: 'Road Trip',
          playlist_url: 'https://open.spotify.com/playlist/sp-road',
          added: 1,
          already_present: 1,
          no_counterpart: [],
        });
      }),
    );
    const user = userEvent.setup();
    renderPage(BackupDetail);

    const umbrella = (await screen.findByText('Umbrella')).closest('tr')!;
    expect(within(umbrella).getByText('yt1')).toBeInTheDocument();
    const rare = screen.getByText('Rare Song').closest('tr')!;
    expect(within(rare).queryByText(/yt/)).not.toBeInTheDocument();
    expect(screen.getByText('1 with YouTube ID')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /csv/i })).toHaveAttribute('href', expect.stringContaining('/api/backups/snapshots/s1/export?format=csv'));

    await user.click(screen.getByRole('button', { name: /restore…/i }));
    const dialog = await screen.findByRole('dialog');
    expect(within(dialog).getByRole('radio', { name: /New YouTube playlist/ })).toHaveTextContent('1 of 2 tracks');
    await user.click(within(dialog).getByRole('button', { name: 'Restore' }));
    await waitFor(() => expect(restores).toEqual([{ mode: 'missing', service: 'spotify' }]));
    expect(await within(dialog).findByText(/1 were already there/)).toBeInTheDocument();
  });
});
