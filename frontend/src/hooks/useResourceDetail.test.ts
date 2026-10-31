// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { cleanup, renderHook, waitFor } from '@testing-library/react';
import { useResourceDetail } from './useResourceDetail';

/** Stubs fetch with a single reply; json() rejects when body is undefined. */
function stubFetch(ok: boolean, status: number, body?: unknown) {
  vi.stubGlobal('fetch', vi.fn(() => Promise.resolve({
    ok,
    status,
    json: () => (body === undefined ? Promise.reject(new SyntaxError('no JSON')) : Promise.resolve(body)),
  } as Response)));
}

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe('useResourceDetail', () => {
  it('loads data', async () => {
    stubFetch(true, 200, { name: 'shop_data' });
    const { result } = renderHook(() => useResourceDetail<{ name: string }>('/api/volumes/shop_data'));
    await waitFor(() => expect(result.current.data).toEqual({ name: 'shop_data' }));
    expect(result.current.error).toBeNull();
  });

  it('shows the error message the server sent', async () => {
    stubFetch(false, 502, { error: "agent rejected the server's credentials" });
    const { result } = renderHook(() => useResourceDetail('/api/containers/abc'));
    await waitFor(() => expect(result.current.error).toBe("agent rejected the server's credentials"));
    expect(result.current.data).toBeNull();
  });

  it('falls back to the HTTP status without a message', async () => {
    stubFetch(false, 500);
    const { result } = renderHook(() => useResourceDetail('/api/containers/abc'));
    await waitFor(() => expect(result.current.error).toBe('HTTP 500'));
  });
});
