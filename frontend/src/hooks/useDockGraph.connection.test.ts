// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { act, cleanup, renderHook } from '@testing-library/react';
import { useDockGraph } from './useDockGraph';
import { WS_IDLE_TIMEOUT } from '../utils/constants';

/** Minimal WebSocket stand-in: the test drives open, messages and close. */
class FakeSocket {
  static instances: FakeSocket[] = [];
  onopen: (() => void) | null = null;
  onmessage: ((e: { data: string }) => void) | null = null;
  onclose: (() => void) | null = null;
  onerror: (() => void) | null = null;
  closed = false;

  constructor(public url: string) {
    FakeSocket.instances.push(this);
  }

  close() {
    this.closed = true;
  }

  open() {
    this.onopen?.();
  }

  receive(msg: object) {
    this.onmessage?.({ data: JSON.stringify(msg) });
  }
}

const heartbeat = { type: 'heartbeat', version: 2 };

describe('useDockGraph connection', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    FakeSocket.instances = [];
    vi.stubGlobal('WebSocket', FakeSocket as unknown as typeof WebSocket);
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
    vi.useRealTimers();
  });

  it('stays connected while heartbeats arrive', () => {
    const { result } = renderHook(() => useDockGraph());
    const ws = FakeSocket.instances[0];
    act(() => ws.open());
    for (let i = 0; i < 12; i++) {
      act(() => {
        vi.advanceTimersByTime(5_000);
        ws.receive(heartbeat);
      });
    }
    expect(FakeSocket.instances).toHaveLength(1);
    expect(ws.closed).toBe(false);
    expect(result.current.connected).toBe(true);
  });

  it('replaces a socket that falls silent', () => {
    const { result } = renderHook(() => useDockGraph());
    const ws = FakeSocket.instances[0];
    act(() => ws.open());
    act(() => ws.receive(heartbeat));

    act(() => vi.advanceTimersByTime(WS_IDLE_TIMEOUT));
    expect(ws.closed).toBe(true);
    expect(result.current.connected).toBe(false);

    act(() => vi.advanceTimersByTime(1_000));
    expect(FakeSocket.instances).toHaveLength(2);
    act(() => FakeSocket.instances[1].open());
    expect(result.current.connected).toBe(true);
  });

  it('gives up on a connection attempt that never opens', () => {
    renderHook(() => useDockGraph());
    act(() => vi.advanceTimersByTime(WS_IDLE_TIMEOUT + 1_000));
    expect(FakeSocket.instances[0].closed).toBe(true);
    expect(FakeSocket.instances).toHaveLength(2);
  });

  it('ignores a late close from a socket it already replaced', () => {
    renderHook(() => useDockGraph());
    const first = FakeSocket.instances[0];
    const lateClose = first.onclose;
    act(() => first.open());
    act(() => vi.advanceTimersByTime(WS_IDLE_TIMEOUT + 1_000));
    const second = FakeSocket.instances[1];
    act(() => second.open());

    act(() => lateClose?.());
    for (let i = 0; i < 6; i++) {
      act(() => {
        vi.advanceTimersByTime(5_000);
        second.receive(heartbeat);
      });
    }
    // The stale close neither schedules a reconnect nor stops the live
    // socket's idle timer from being kept alive.
    expect(FakeSocket.instances).toHaveLength(2);
    expect(second.closed).toBe(false);
  });

  it('reconnects after a normal close', () => {
    const { result } = renderHook(() => useDockGraph());
    const ws = FakeSocket.instances[0];
    act(() => ws.open());
    act(() => ws.onclose?.());
    expect(result.current.connected).toBe(false);
    act(() => vi.advanceTimersByTime(1_000));
    expect(FakeSocket.instances).toHaveLength(2);
  });
});
