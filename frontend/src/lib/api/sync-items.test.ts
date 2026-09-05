import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { syncItemsAPI } from './sync-items';
import { APIClientError } from './client';

describe('syncItemsAPI', () => {
  const fetchMock = vi.fn();

  beforeEach(() => {
    vi.stubGlobal('fetch', fetchMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    fetchMock.mockReset();
  });

  it('execute maps 409 conflict from Echo message body', async () => {
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ message: 'sync item is not executable' }), {
        status: 409,
        headers: { 'Content-Type': 'application/json' },
      }),
    );

    await expect(syncItemsAPI.execute('sync-done-1')).rejects.toMatchObject({
      code: 'conflict',
      message: 'sync item is not executable',
    } satisfies Partial<APIClientError>);
  });

  it('list builds filter query params', async () => {
    fetchMock.mockResolvedValue(
      new Response(
        JSON.stringify({ page: 1, perPage: 20, totalItems: 0, totalPages: 0, items: [] }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      ),
    );

    await syncItemsAPI.getList({ status: 'pending', service: 'youtube' });

    const calledUrl = fetchMock.mock.calls[0][0] as string;
    expect(calledUrl).toContain('status=pending');
    expect(calledUrl).toContain('service=youtube');
  });
});
