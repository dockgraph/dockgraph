// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { ThemeProvider } from '../../theme';
import { CommonLogs } from './CommonLogs';
import { STANDALONE_STACK } from '../../utils/stack';
import type { ReactNode } from 'react';

class StubEventSource {
  onopen: (() => void) | null = null;
  onmessage: ((e: MessageEvent) => void) | null = null;
  onerror: (() => void) | null = null;
  close = vi.fn();
  constructor(public url: string) {}
}

const wrap = (ui: ReactNode) => render(<ThemeProvider>{ui}</ThemeProvider>);

describe('CommonLogs', () => {
  beforeEach(() => {
    vi.stubGlobal('EventSource', StubEventSource as unknown as typeof EventSource);
    vi.stubGlobal('fetch', vi.fn(() =>
      Promise.resolve(new Response(JSON.stringify({
        lines: [
          { container: 'web', stream: 'stdout', line: 'hello world', timestamp: '2026-06-11T10:00:00.000Z' },
          { container: 'db', stream: 'stderr', line: 'boom error', timestamp: '2026-06-11T10:00:01.000Z' },
        ],
      }), { status: 200 })),
    ));
  });
  afterEach(() => { cleanup(); vi.unstubAllGlobals(); vi.restoreAllMocks(); });

  it('renders aggregated lines with container badges and filters by text', async () => {
    wrap(<CommonLogs active onOpenContainer={vi.fn()} />);
    expect(await screen.findByText('hello world')).toBeTruthy();
    expect(screen.getByText('boom error')).toBeTruthy();

    fireEvent.change(screen.getByPlaceholderText('Filter logs'), { target: { value: 'boom' } });
    expect(screen.queryByText('hello world')).toBeNull();
    expect(screen.getByText('boom error')).toBeTruthy();
  });

  it('clicking a container badge calls onOpenContainer', async () => {
    const onOpenContainer = vi.fn();
    wrap(<CommonLogs active onOpenContainer={onOpenContainer} />);
    fireEvent.click(await screen.findByText('web'));
    expect(onOpenContainer).toHaveBeenCalledWith('web');
  });

  it('passes ?stack= to the history and stream endpoints', async () => {
    wrap(<CommonLogs active onOpenContainer={vi.fn()} stack="shop" />);
    await screen.findByText('hello world');
    const url = vi.mocked(fetch).mock.calls[0][0] as string;
    expect(url).toMatch(/^\/api\/logs\/history\?stack=shop&limit=\d+$/);
  });

  it('standalone scope filters lines client-side by workload name', async () => {
    wrap(<CommonLogs active onOpenContainer={vi.fn()} stack={STANDALONE_STACK} scopeNames={new Set(['db'])} />);
    expect(await screen.findByText('boom error')).toBeTruthy();
    expect(screen.queryByText('hello world')).toBeNull();
    expect(vi.mocked(fetch).mock.calls[0][0]).not.toContain('stack=');
  });
});
