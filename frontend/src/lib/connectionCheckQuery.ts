import { APIClientError } from './api/client';

/** React Query retry policy for "is this OAuth provider connected?" probes. */
export function shouldRetryConnectionCheck(failureCount: number, error: unknown): boolean {
  if (error instanceof APIClientError && error.status === 401) {
    return false;
  }
  if (error instanceof APIClientError && error.status >= 500) {
    return failureCount < 1;
  }
  return failureCount < 2;
}
