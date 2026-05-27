import { describe, it, expect } from 'vitest';
import { ApiError } from './api';
import { shouldRetryConnectionCheck } from './connectionCheckQuery';

describe('shouldRetryConnectionCheck', () => {
  it('does not retry when unauthenticated', () => {
    const error = new ApiError(401, 'spotify account not connected');
    expect(shouldRetryConnectionCheck(0, error)).toBe(false);
    expect(shouldRetryConnectionCheck(2, error)).toBe(false);
  });

  it('retries server errors at most once', () => {
    const error = new ApiError(502, 'bad gateway');
    expect(shouldRetryConnectionCheck(0, error)).toBe(true);
    expect(shouldRetryConnectionCheck(1, error)).toBe(false);
  });

  it('retries transient client errors at most twice', () => {
    const error = new ApiError(429, 'rate limited');
    expect(shouldRetryConnectionCheck(0, error)).toBe(true);
    expect(shouldRetryConnectionCheck(1, error)).toBe(true);
    expect(shouldRetryConnectionCheck(2, error)).toBe(false);
  });
});
