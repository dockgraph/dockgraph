// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { ThemeProvider } from '../../theme';
import { LogStream } from './LogStream';
import type { LogLine } from '../../types/stats';
import type { ReactNode } from 'react';

afterEach(() => cleanup());

const wrap = (ui: ReactNode) => render(<ThemeProvider>{ui}</ThemeProvider>);

const lines: LogLine[] = [
  { id: 1, stream: 'stdout', text: 'worker started', timestamp: '2026-06-10T10:00:00.000000000Z' },
  { id: 2, stream: 'stderr', text: 'connection error reset', timestamp: '2026-06-10T10:00:01.000000000Z' },
  { id: 3, stream: 'stdout', text: 'upstream error timeout', timestamp: '2026-06-10T10:00:02.000000000Z' },
];

const baseProps = {
  lines,
  connected: true,
  loading: false,
  loadingMore: false,
  hasMore: false,
  loadMore: vi.fn(),
};

describe('LogStream container badge', () => {
  const tagged: LogLine[] = [
    { id: 10, stream: 'stdout', text: 'hello', timestamp: '2026-06-11T10:00:00Z', container: 'web' },
  ];

  it('renders the container badge and fires onContainerClick', () => {
    const onContainerClick = vi.fn();
    wrap(<LogStream {...baseProps} lines={tagged} showContainer onContainerClick={onContainerClick} onLineMenu={vi.fn()} />);
    const badge = screen.getByText('web');
    expect(badge).toBeTruthy();
    fireEvent.click(badge);
    expect(onContainerClick).toHaveBeenCalledWith('web');
  });

  it('renders the container badge at the row text size, not the larger UA button default', () => {
    wrap(<LogStream {...baseProps} lines={tagged} showContainer onContainerClick={vi.fn()} onLineMenu={vi.fn()} />);
    const badge = screen.getByText('web');
    // Buttons default to the UA font size (~13px); it must be pinned to the 10px row text.
    expect(badge.style.fontSize).toBe('10px');
  });

  it('fires onLineMenu from the row kebab', () => {
    const onLineMenu = vi.fn();
    wrap(<LogStream {...baseProps} lines={tagged} showContainer onContainerClick={vi.fn()} onLineMenu={onLineMenu} />);
    fireEvent.click(screen.getByLabelText('Row actions'));
    expect(onLineMenu).toHaveBeenCalled();
  });
});

describe('LogStream', () => {
  it('renders log lines', () => {
    wrap(<LogStream {...baseProps} />);
    expect(screen.getByText('worker started')).toBeTruthy();
  });

  it('shows the loading placeholder when loading with no lines', () => {
    wrap(<LogStream {...baseProps} lines={[]} loading={true} />);
    expect(screen.getByText('Loading logs...')).toBeTruthy();
  });

  it('reports the match counter as current/total when a search is supplied', () => {
    wrap(<LogStream {...baseProps} search="error" onSearchChange={vi.fn()} showSearch />);
    // 2 lines contain "error" -> "1/2" initially
    expect(screen.getByText('1/2')).toBeTruthy();
  });

  it('cycles to the next match with the ▼ button', () => {
    wrap(<LogStream {...baseProps} search="error" onSearchChange={vi.fn()} showSearch />);
    fireEvent.click(screen.getByLabelText('Next match'));
    expect(screen.getByText('2/2')).toBeTruthy();
  });

  it('calls onSearchChange as the user types', () => {
    const onSearchChange = vi.fn();
    wrap(<LogStream {...baseProps} search="" onSearchChange={onSearchChange} showSearch />);
    const input = screen.getByPlaceholderText('Search logs') as HTMLInputElement;
    fireEvent.change(input, { target: { value: 'err' } });
    expect(onSearchChange).toHaveBeenCalledWith('err');
  });
});
