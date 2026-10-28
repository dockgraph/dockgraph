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

describe('LogStream scrolling', () => {
  // jsdom has no layout: give every row of the scroll box a fixed height,
  // stacked by its index, and the box a 100px viewport with a clamped
  // scrollTop, so scroll positions can be asserted.
  const ROW = 20;
  const VIEW = 100;
  const proto = HTMLElement.prototype;
  const saved = {
    offsetTop: Object.getOwnPropertyDescriptor(proto, 'offsetTop'),
    offsetHeight: Object.getOwnPropertyDescriptor(proto, 'offsetHeight'),
  };

  const rowIndex = (el: HTMLElement) => (el.parentElement ? Array.prototype.indexOf.call(el.parentElement.children, el) : 0);

  function fakeLayout() {
    Object.defineProperty(proto, 'offsetTop', { configurable: true, get() { return rowIndex(this) * ROW; } });
    Object.defineProperty(proto, 'offsetHeight', { configurable: true, get() { return ROW; } });
  }

  afterEach(() => {
    for (const [key, desc] of Object.entries(saved)) {
      if (desc) Object.defineProperty(proto, key, desc);
    }
  });

  /** Renders the stream and gives its scroll box a browser-like scrollTop. */
  function renderScrollable(initial: LogLine[], props: Partial<React.ComponentProps<typeof LogStream>> = {}) {
    fakeLayout();
    let top = 0;
    const view = (ui: LogLine[]) => (
      <ThemeProvider>
        <LogStream {...baseProps} hasMore {...props} lines={ui} />
      </ThemeProvider>
    );
    const utils = render(view([]));
    const box = screen.getByTestId('log-scroll');
    const max = () => Math.max(0, box.children.length * ROW - VIEW);
    Object.defineProperty(box, 'clientHeight', { configurable: true, get: () => VIEW });
    Object.defineProperty(box, 'scrollHeight', { configurable: true, get: () => box.children.length * ROW });
    Object.defineProperty(box, 'scrollTop', {
      configurable: true,
      get: () => top,
      set: (v: number) => { top = Math.min(Math.max(0, v), max()); },
    });
    utils.rerender(view(initial));
    return {
      box,
      max,
      update: (next: LogLine[]) => utils.rerender(view(next)),
      /** Scrolls like the user would: moves the box, then fires the event. */
      scrollTo: (v: number) => { box.scrollTop = v; fireEvent.scroll(box); },
    };
  }

  const range = (from: number, to: number): LogLine[] =>
    Array.from({ length: to - from + 1 }, (_, i) => ({ id: from + i, stream: 'stdout' as const, text: `line ${from + i}` }));

  /** Distance of a line's row from the top of the viewport. */
  const offsetInView = (box: HTMLElement, text: string) =>
    (screen.getByText(text).closest('[data-line-id]') as HTMLElement).offsetTop - box.scrollTop;

  it('follows new lines while at the bottom', () => {
    const s = renderScrollable(range(1, 20));
    expect(s.box.scrollTop).toBe(s.max());
    s.update(range(1, 25));
    expect(s.box.scrollTop).toBe(s.max());
  });

  it('holds still when new lines arrive while scrolled up', () => {
    const s = renderScrollable(range(1, 20));
    s.scrollTo(100);
    s.update(range(1, 25));
    expect(s.box.scrollTop).toBe(100);
  });

  it('holds the viewed lines still when the buffer drops the oldest ones', () => {
    const s = renderScrollable(range(1, 20));
    s.scrollTo(200); // line 11 at the top of the viewport
    s.update(range(6, 30)); // 1-5 dropped, 21-30 appended
    expect(offsetInView(s.box, 'line 11')).toBe(0);
  });

  it('holds the viewed lines still when older lines are prepended', () => {
    const s = renderScrollable(range(11, 30));
    s.scrollTo(60); // line 14 at the top of the viewport
    s.update(range(1, 30));
    expect(offsetInView(s.box, 'line 14')).toBe(0);
  });

  it('tells the log source whether the reader follows the tail', () => {
    const onFollowChange = vi.fn();
    const s = renderScrollable(range(1, 20), { onFollowChange });
    s.scrollTo(100);
    expect(onFollowChange).toHaveBeenLastCalledWith(false);
    s.scrollTo(s.max());
    expect(onFollowChange).toHaveBeenLastCalledWith(true);
  });

  it('catches up with newer lines on reaching the bottom of a detached buffer', () => {
    const loadNewer = vi.fn();
    const s = renderScrollable(range(1, 20), { hasNewer: true, loadNewer });
    expect(screen.getByText('Newer logs available — scroll to the bottom to catch up')).toBeTruthy();
    s.scrollTo(100);
    expect(loadNewer).not.toHaveBeenCalled();
    s.scrollTo(s.max());
    expect(loadNewer).toHaveBeenCalledTimes(1);
  });

  it('does not ask for newer lines twice while they load', () => {
    const loadNewer = vi.fn();
    const s = renderScrollable(range(1, 20), { hasNewer: true, loadingNewer: true, loadNewer });
    expect(screen.getByText('Loading newer logs...')).toBeTruthy();
    s.scrollTo(s.max());
    expect(loadNewer).not.toHaveBeenCalled();
  });

  it('resumes following when a wheel scroll heads past the end, before the scroll lands', () => {
    // Under fast logs a smooth wheel scroll lands short of a bottom that
    // keeps moving; the wheel's own target decides instead.
    const s = renderScrollable(range(1, 20));
    s.scrollTo(100);
    fireEvent.wheel(s.box, { deltaY: 10_000 });
    s.update(range(1, 30));
    expect(s.box.scrollTop).toBe(s.max());
  });

  it('pins to the bottom as soon as a wheel scroll heads past the end', () => {
    // Setting the position aborts the browser's smooth scroll (CSSOM View),
    // so it can't land short and leave following half-resumed.
    const s = renderScrollable(range(1, 20));
    s.scrollTo(100);
    fireEvent.wheel(s.box, { deltaY: 10_000 });
    expect(s.box.scrollTop).toBe(s.max());
  });

  it('does not resume following on a wheel scroll that stops short of the end', () => {
    const s = renderScrollable(range(1, 20));
    s.scrollTo(100);
    fireEvent.wheel(s.box, { deltaY: 40 });
    s.update(range(1, 25));
    expect(s.box.scrollTop).toBe(100);
  });

  it('follows again after scrolling back to the bottom', () => {
    const s = renderScrollable(range(1, 20));
    s.scrollTo(100);
    s.scrollTo(s.max());
    s.update(range(1, 25));
    expect(s.box.scrollTop).toBe(s.max());
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
