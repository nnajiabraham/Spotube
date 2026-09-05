import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { APIClient, APIClientError } from './client';

describe('APIClient error handling', () => {
  const fetchMock = vi.fn();

  beforeEach(() => {
    vi.stubGlobal('fetch', fetchMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    fetchMock.mockReset();
  });

  it('maps Echo-style 401 message bodies to unauthorized', async () => {
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ message: 'spotify account not connected' }), {
        status: 401,
        headers: { 'Content-Type': 'application/json' },
      }),
    );

    const client = new APIClient('http://localhost:8090');

    await expect(client.get('/api/auth/spotify/playlists')).rejects.toMatchObject({
      code: 'unauthorized',
      message: 'spotify account not connected',
    } satisfies Partial<APIClientError>);
  });

  it('maps structured API error envelopes', async () => {
    fetchMock.mockResolvedValue(
      new Response(
        JSON.stringify({
          error: { code: 'unauthorized', message: 'Not authenticated with Spotify' },
        }),
        { status: 401, headers: { 'Content-Type': 'application/json' } },
      ),
    );

    const client = new APIClient('http://localhost:8090');

    await expect(client.get('/api/auth/spotify/playlists')).rejects.toMatchObject({
      code: 'unauthorized',
      message: 'Not authenticated with Spotify',
    });
  });
});
