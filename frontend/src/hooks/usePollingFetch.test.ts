// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { cleanup, renderHook, waitFor } from '@testing-library/react';
import { usePollingFetch } from './usePollingFetch';

type Reply = { ok: boolean; status?: number; body?: unknown } | 'hang';

/** Stubs fetch with one scripted reply per URL. */
function stubFetch(replies: Record<string, Reply>) {
  const fetchMock = vi.fn((url: string, init?: RequestInit) => {
    const reply = replies[url];
    if (reply === 'hang' || reply === undefined) {
      return new Promise<Response>((_, reject) => {
        init?.signal?.addEventListener('abort', () => reject(new DOMException('aborted', 'AbortError')));
      });
    }
    return Promise.resolve({
      ok: reply.ok,
      status: reply.status ?? 200,
      json: () => Promise.resolve(reply.body),
    } as Response);
  });
  vi.stubGlobal('fetch', fetchMock);
  return fetchMock;
}

beforeEach(() => vi.useRealTimers());
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe('usePollingFetch', () => {
  it('loads data for a URL', async () => {
    stubFetch({ '/a': { ok: true, body: { v: 'a' } } });
    const { result } = renderHook(() => usePollingFetch<{ v: string }>('/a', 60_000));
    await waitFor(() => expect(result.current.data).toEqual({ v: 'a' }));
    expect(result.current.loading).toBe(false);
  });

  it('never shows the previous URL data while the new URL loads', async () => {
    stubFetch({ '/a': { ok: true, body: { v: 'a' } }, '/b': 'hang' });
    const { result, rerender } = renderHook(({ url }) => usePollingFetch<{ v: string }>(url, 60_000), {
      initialProps: { url: '/a' },
    });
    await waitFor(() => expect(result.current.data).toEqual({ v: 'a' }));

    rerender({ url: '/b' });
    // Also covers the first render for /b, before its effect has run.
    expect(result.current.data).toBeNull();
    expect(result.current.loading).toBe(true);
  });

  it('never shows the previous URL data after the new URL fails', async () => {
    stubFetch({ '/a': { ok: true, body: { v: 'a' } }, '/b': { ok: false, status: 500 } });
    const { result, rerender } = renderHook(({ url }) => usePollingFetch<{ v: string }>(url, 60_000), {
      initialProps: { url: '/a' },
    });
    await waitFor(() => expect(result.current.data).toEqual({ v: 'a' }));

    rerender({ url: '/b' });
    await waitFor(() => expect(result.current.error).toBe('HTTP 500'));
    expect(result.current.data).toBeNull();
  });

  it('keeps the same URL data when a later poll fails', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const fetchMock = stubFetch({ '/a': { ok: true, body: { v: 'a' } } });
    const { result } = renderHook(() => usePollingFetch<{ v: string }>('/a', 1_000));
    await waitFor(() => expect(result.current.data).toEqual({ v: 'a' }));

    fetchMock.mockImplementation(() => Promise.resolve({ ok: false, status: 503, json: () => Promise.resolve(null) } as Response));
    await vi.advanceTimersByTimeAsync(1_000);
    await waitFor(() => expect(result.current.error).toBe('HTTP 503'));
    expect(result.current.data).toEqual({ v: 'a' });
  });

  it('never starts a poll while the previous one is pending', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const pending: ((body: unknown) => void)[] = [];
    const fetchMock = vi.fn(() => new Promise<Response>((resolve) => {
      pending.push((body) => resolve({ ok: true, status: 200, json: () => Promise.resolve(body) } as Response));
    }));
    vi.stubGlobal('fetch', fetchMock);
    const { result } = renderHook(() => usePollingFetch<{ v: number }>('/a', 1_000));

    // A slow first request spans several intervals: no second one starts.
    await vi.advanceTimersByTimeAsync(3_500);
    expect(fetchMock).toHaveBeenCalledTimes(1);

    pending[0]({ v: 1 });
    await waitFor(() => expect(result.current.data).toEqual({ v: 1 }));
    // The next poll starts one interval after the previous one settled.
    await vi.advanceTimersByTimeAsync(1_000);
    expect(fetchMock).toHaveBeenCalledTimes(2);
    pending[1]({ v: 2 });
    await waitFor(() => expect(result.current.data).toEqual({ v: 2 }));
  });

  it('shows data again when switching back to a URL', async () => {
    stubFetch({ '/a': { ok: true, body: { v: 'a' } }, '/b': { ok: true, body: { v: 'b' } } });
    const { result, rerender } = renderHook(({ url }) => usePollingFetch<{ v: string }>(url, 60_000), {
      initialProps: { url: '/a' },
    });
    await waitFor(() => expect(result.current.data).toEqual({ v: 'a' }));
    rerender({ url: '/b' });
    await waitFor(() => expect(result.current.data).toEqual({ v: 'b' }));
    rerender({ url: '/a' });
    expect(result.current.data).toBeNull();
    await waitFor(() => expect(result.current.data).toEqual({ v: 'a' }));
  });

  it('is empty without a URL', () => {
    const fetchMock = stubFetch({});
    const { result } = renderHook(() => usePollingFetch(null, 60_000));
    expect(result.current).toEqual({ data: null, loading: false, error: null });
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
