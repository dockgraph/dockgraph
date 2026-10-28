// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { act, renderHook } from '@testing-library/react';
import { useLogs } from './useLogs';

/** Minimal EventSource stub so the hook can open a live tail without a real server. */
class StubEventSource {
  static opened: StubEventSource[] = [];
  onopen: (() => void) | null = null;
  onmessage: ((e: MessageEvent) => void) | null = null;
  onerror: (() => void) | null = null;
  close = vi.fn();
  constructor(public url: string) {
    StubEventSource.opened.push(this);
  }
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

describe('useLogs with a full buffer', () => {
  // A 4-line buffer: the latest page fills it, so loading older lines must
  // give up the newest ones, and live lines must not land after a hole.
  const ts = (n: number) => `2026-06-10T10:00:${String(n).padStart(2, '0')}.000000000Z`;
  const entry = (text: string, n: number) => ({ stream: 'stdout', line: text, timestamp: ts(n) });
  let latest: ReturnType<typeof entry>[];

  beforeEach(() => {
    StubEventSource.opened = [];
    vi.stubGlobal('EventSource', StubEventSource as unknown as typeof EventSource);
    latest = [entry('l1', 11), entry('l2', 12), entry('l3', 13), entry('l4', 14)];
    vi.stubGlobal('fetch', vi.fn((url: string) => {
      const lines = url.includes('before=') ? [entry('o1', 1), entry('o2', 2)] : latest;
      return Promise.resolve(new Response(JSON.stringify({ lines }), { status: 200 }));
    }));
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  const settle = () => act(async () => { await sleep(60); });
  const texts = (r: { current: { lines: { text: string }[] } }) => r.current.lines.map((l) => l.text);
  const live = (text: string, n: number) =>
    StubEventSource.opened.at(-1)!.onmessage!({ data: JSON.stringify(entry(text, n)) } as MessageEvent);

  function renderLogs() {
    return renderHook(() => useLogs({ historyUrl: '/h', streamUrl: '/s', active: true, pageSize: 4, bufferSize: 4 }));
  }

  it('follows the live tail, dropping the oldest lines', async () => {
    const { result } = renderLogs();
    await settle();
    act(() => live('l5', 15));
    await settle();
    expect(texts(result)).toEqual(['l2', 'l3', 'l4', 'l5']);
    expect(result.current.hasNewer).toBe(false);
  });

  it('stops the live tail after loading older lines into a full buffer, instead of leaving a gap', async () => {
    const { result } = renderLogs();
    await settle();
    act(() => result.current.loadMore());
    await settle();
    // l3 and l4 had to go to make room for the older page.
    expect(texts(result)).toEqual(['o1', 'o2', 'l1', 'l2']);

    act(() => live('l5', 15));
    await settle();
    expect(texts(result)).toEqual(['o1', 'o2', 'l1', 'l2']);
    expect(result.current.hasNewer).toBe(true);
  });

  it('catches up with the latest lines and the live tail on loadNewer', async () => {
    const { result } = renderLogs();
    await settle();
    act(() => result.current.loadMore());
    await settle();

    latest = [entry('l3', 13), entry('l4', 14), entry('l5', 15), entry('l6', 16)];
    act(() => result.current.loadNewer());
    await settle();
    expect(texts(result)).toEqual(['l3', 'l4', 'l5', 'l6']);
    expect(result.current.hasNewer).toBe(false);
    // The tail resumes after the newest line shown, so nothing is skipped.
    expect(StubEventSource.opened.at(-1)!.url).toBe(`/s?since=${encodeURIComponent(ts(16))}`);
    act(() => live('l7', 17));
    await settle();
    expect(texts(result)).toEqual(['l4', 'l5', 'l6', 'l7']);
  });

  it('drops a batch the replaced stream had queued, as the new page already holds it', async () => {
    const { result } = renderLogs();
    await settle();
    act(() => result.current.loadMore());
    await settle();
    const stale = StubEventSource.opened.at(-1)!;
    latest = [entry('l3', 13), entry('l4', 14), entry('l5', 15), entry('l6', 16)];
    act(() => result.current.loadNewer());
    // Queued on the old stream before its replacement was opened.
    act(() => stale.onmessage!({ data: JSON.stringify(entry('l5', 15)) } as MessageEvent));
    await settle();
    expect(texts(result)).toEqual(['l3', 'l4', 'l5', 'l6']);
  });

  it('keeps the lines a reader scrolled up to while the live tail fills the buffer', async () => {
    const { result } = renderLogs();
    await settle();
    act(() => result.current.onFollowChange(false));
    act(() => live('l5', 15));
    await settle();
    expect(texts(result)).toEqual(['l1', 'l2', 'l3', 'l4']);
    expect(result.current.hasNewer).toBe(true);
  });
});
