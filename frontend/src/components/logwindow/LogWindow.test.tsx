// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { ThemeProvider } from '../../theme';
import { LogWindow } from './LogWindow';
import type { LogWindowState } from '../../hooks/logWindowsState';
import type { ReactNode } from 'react';

class StubEventSource {
  onopen: (() => void) | null = null;
  onmessage: ((e: MessageEvent) => void) | null = null;
  onerror: (() => void) | null = null;
  close = vi.fn();
  constructor(public url: string) {}
}

const win: LogWindowState = {
  id: 'lw-1',
  tabs: [{ containerId: 'nginx', title: 'nginx-1', search: '' }],
  activeTab: 0,
  x: 100, y: 100, w: 460, h: 340, z: 1, minimized: false,
};

const handlers = {
  onFocus: vi.fn(), onClose: vi.fn(), onCloseTab: vi.fn(), onMinimize: vi.fn(),
  onMove: vi.fn(), onResize: vi.fn(), onSetActiveTab: vi.fn(), onSetSearch: vi.fn(),
  onTitleClick: vi.fn(), onDragMove: vi.fn(), onDragEnd: vi.fn(), onTabDetach: vi.fn(),
};

const wrap = (ui: ReactNode) => render(<ThemeProvider>{ui}</ThemeProvider>);

describe('LogWindow', () => {
  beforeEach(() => {
    vi.stubGlobal('EventSource', StubEventSource as unknown as typeof EventSource);
    vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(new Response(JSON.stringify({ lines: [] }), { status: 200 }))));
  });
  afterEach(() => { cleanup(); vi.unstubAllGlobals(); vi.restoreAllMocks(); });

  it('renders the container title', () => {
    wrap(<LogWindow win={win} flashed={false} {...handlers} />);
    expect(screen.getByText('nginx-1')).toBeTruthy();
  });

  it('fires onClose and onMinimize from the control buttons', () => {
    wrap(<LogWindow win={win} flashed={false} {...handlers} />);
    fireEvent.click(screen.getByLabelText('Close window'));
    expect(handlers.onClose).toHaveBeenCalledWith('lw-1');
    fireEvent.click(screen.getByLabelText('Minimize window'));
    expect(handlers.onMinimize).toHaveBeenCalledWith('lw-1');
  });

  it('fires onTitleClick when the container name is clicked', () => {
    wrap(<LogWindow win={win} flashed={false} {...handlers} />);
    fireEvent.click(screen.getByText('nginx-1'));
    expect(handlers.onTitleClick).toHaveBeenCalledWith('nginx');
  });
});
