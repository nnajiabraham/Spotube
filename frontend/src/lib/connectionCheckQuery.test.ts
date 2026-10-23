import { describe, it, expect } from 'vitest';
import { APIClientError } from './api/client';
import { shouldRetryConnectionCheck } from './connectionCheckQuery';

describe('shouldRetryConnectionCheck', () => {
  it('does not retry when unauthenticated', () => {
    const error = new APIClientError('http_error', 'spotify account not connected', 401);
    expect(shouldRetryConnectionCheck(0, error)).toBe(false);
    expect(shouldRetryConnectionCheck(2, error)).toBe(false);
  });

  it('retries server errors at most once', () => {
    const error = new APIClientError('http_error', 'bad gateway', 502);
    expect(shouldRetryConnectionCheck(0, error)).toBe(true);
    expect(shouldRetryConnectionCheck(1, error)).toBe(false);
  });

  it('retries transient client errors at most twice', () => {
    const error = new APIClientError('http_error', 'rate limited', 429);
    expect(shouldRetryConnectionCheck(0, error)).toBe(true);
    expect(shouldRetryConnectionCheck(1, error)).toBe(true);
    expect(shouldRetryConnectionCheck(2, error)).toBe(false);
  });
});
