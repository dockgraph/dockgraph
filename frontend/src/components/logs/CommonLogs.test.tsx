// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { ThemeProvider } from '../../theme';
import { CommonLogs } from './CommonLogs';
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
});
