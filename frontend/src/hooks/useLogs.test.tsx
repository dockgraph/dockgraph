// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { act, renderHook } from '@testing-library/react';
import { useLogs } from './useLogs';

/** Minimal EventSource stub so the hook can open a live tail without a real server. */
class StubEventSource {
  onopen: (() => void) | null = null;
  onmessage: ((e: MessageEvent) => void) | null = null;
  onerror: (() => void) | null = null;
  close = vi.fn();
  constructor(public url: string) {}
}

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

describe('useLogs', () => {
  beforeEach(() => {
    vi.stubGlobal('EventSource', StubEventSource as unknown as typeof EventSource);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it('keeps history lines that resolve before the reset frame fires', async () => {
    // Reproduce the real-world timing: the local backend answers far faster
    // than one animation frame, so the fetch resolves before the deferred
    // state reset. The reset must not clobber the freshly-fetched lines.
    const history = {
      lines: [
        { stream: 'stdout', line: 'first', timestamp: '2026-06-10T10:00:00.000000000Z' },
        { stream: 'stdout', line: 'second', timestamp: '2026-06-10T10:00:01.000000000Z' },
      ],
    };
    vi.stubGlobal(
      'fetch',
      vi.fn(() => Promise.resolve(new Response(JSON.stringify(history), { status: 200 }))),
    );

    const { result } = renderHook(() =>
      useLogs({
        historyUrl: '/api/containers/abc/logs/history',
        streamUrl: '/api/containers/abc/logs',
        active: true,
      }),
    );

    // Let both the fetch microtask and the requestAnimationFrame reset settle.
    await act(async () => {
      await sleep(60);
    });

    expect(result.current.lines.map((l) => l.text)).toEqual(['first', 'second']);
    expect(result.current.loading).toBe(false);
  });
});
