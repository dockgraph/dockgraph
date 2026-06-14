import { describe, it, expect } from 'vitest';
import {
  initialWindowsState,
  openLogs,
  closeWindow,
  focusWindow,
  moveWindow,
  resizeWindow,
  minimizeWindow,
  restoreWindow,
  setActiveTab,
  setSearch,
  mergeWindows,
  detachTab,
  closeTab,
} from './logWindowsState';

const VP = { width: 1200, height: 800, headerHeight: 50, dockHeight: 36 };

describe('logWindowsState: open / close / focus', () => {
  it('opens a new window with one tab for a container', () => {
    const s = openLogs(initialWindowsState, 'nginx', 'nginx-1', VP);
    expect(s.windows).toHaveLength(1);
    expect(s.windows[0].tabs).toEqual([{ containerId: 'nginx', title: 'nginx-1', search: '' }]);
    expect(s.windows[0].activeTab).toBe(0);
    expect(s.windows[0].minimized).toBe(false);
  });

  it('focus-existing: re-opening the same container does not duplicate, brings it to front', () => {
    let s = openLogs(initialWindowsState, 'nginx', 'nginx-1', VP);
    const firstId = s.windows[0].id;
    s = openLogs(s, 'redis', 'redis-1', VP);
    const topBefore = s.windows.find((w) => w.id === firstId)!.z;
    s = openLogs(s, 'nginx', 'nginx-1', VP); // already open
    expect(s.windows).toHaveLength(2); // not duplicated
    const win = s.windows.find((w) => w.id === firstId)!;
    expect(win.z).toBeGreaterThan(topBefore); // brought to front
    expect(win.activeTab).toBe(0); // its tab is active
    expect(win.minimized).toBe(false); // restored if it had been minimized
  });

  it('cascades new windows so they do not stack exactly', () => {
    let s = openLogs(initialWindowsState, 'a', 'a', VP);
    s = openLogs(s, 'b', 'b', VP);
    expect(s.windows[1].x).not.toBe(s.windows[0].x);
    expect(s.windows[1].y).not.toBe(s.windows[0].y);
  });

  it('closeWindow removes the window', () => {
    let s = openLogs(initialWindowsState, 'a', 'a', VP);
    const id = s.windows[0].id;
    s = closeWindow(s, id);
    expect(s.windows).toHaveLength(0);
  });

  it('focusWindow raises z above all others', () => {
    let s = openLogs(initialWindowsState, 'a', 'a', VP);
    s = openLogs(s, 'b', 'b', VP);
    const aId = s.windows[0].id;
    s = focusWindow(s, aId);
    const a = s.windows.find((w) => w.id === aId)!;
    const others = s.windows.filter((w) => w.id !== aId);
    expect(Math.max(...others.map((w) => w.z))).toBeLessThan(a.z);
  });
});

describe('logWindowsState: move / resize / minimize / tabs / search', () => {
  const open = () => openLogs(initialWindowsState, 'a', 'a', VP);

  it('moveWindow clamps below the header', () => {
    let s = open();
    const id = s.windows[0].id;
    s = moveWindow(s, id, 500, -200, VP); // y above header
    expect(s.windows[0].x).toBe(500);
    expect(s.windows[0].y).toBe(VP.headerHeight); // clamped
  });

  it('resizeWindow enforces minimum size', () => {
    let s = open();
    const id = s.windows[0].id;
    s = resizeWindow(s, id, 10, 10);
    expect(s.windows[0].w).toBeGreaterThanOrEqual(280);
    expect(s.windows[0].h).toBeGreaterThanOrEqual(160);
  });

  it('minimize then restore toggles the flag', () => {
    let s = open();
    const id = s.windows[0].id;
    s = minimizeWindow(s, id);
    expect(s.windows[0].minimized).toBe(true);
    s = restoreWindow(s, id);
    expect(s.windows[0].minimized).toBe(false);
  });

  it('setActiveTab and setSearch update the right tab', () => {
    let s = openLogs(initialWindowsState, 'a', 'a', VP);
    const id = s.windows[0].id;
    s = setSearch(s, id, 0, 'error');
    expect(s.windows[0].tabs[0].search).toBe('error');
    // setActiveTab clamps to valid range
    s = setActiveTab(s, id, 5);
    expect(s.windows[0].activeTab).toBe(0);
  });
});

describe('logWindowsState: merge / detach / closeTab', () => {
  const twoWindows = () => {
    let s = openLogs(initialWindowsState, 'a', 'a', VP);
    s = openLogs(s, 'b', 'b', VP);
    return s;
  };

  it('mergeWindows moves source tabs into target and removes source', () => {
    let s = twoWindows();
    const [a, b] = s.windows;
    s = mergeWindows(s, a.id, b.id);
    expect(s.windows).toHaveLength(1);
    const merged = s.windows[0];
    expect(merged.id).toBe(b.id);
    expect(merged.tabs.map((t) => t.containerId).sort()).toEqual(['a', 'b']);
    expect(merged.activeTab).toBe(merged.tabs.findIndex((t) => t.containerId === 'a')); // newly added tab active
  });

  it('detachTab pops a tab out of a merged window into a new window', () => {
    let s = twoWindows();
    const [a, b] = s.windows;
    s = mergeWindows(s, a.id, b.id); // b now has [b, a]
    const mergedId = s.windows[0].id;
    s = detachTab(s, mergedId, 0, 300, 300); // detach first tab
    expect(s.windows).toHaveLength(2);
    const detached = s.windows.find((w) => w.id !== mergedId)!;
    expect(detached.tabs).toHaveLength(1);
    expect(s.windows.find((w) => w.id === mergedId)!.tabs).toHaveLength(1);
  });

  it('detachTab is a no-op when the window has a single tab', () => {
    let s = openLogs(initialWindowsState, 'solo', 'solo', VP);
    const id = s.windows[0].id;
    s = detachTab(s, id, 0, 300, 300);
    expect(s.windows).toHaveLength(1);
  });

  it('closeTab removes a tab; closing the last tab removes the window', () => {
    let s = twoWindows();
    const [a, b] = s.windows;
    s = mergeWindows(s, a.id, b.id);
    const mergedId = s.windows[0].id;
    s = closeTab(s, mergedId, 0);
    expect(s.windows[0].tabs).toHaveLength(1);
    s = closeTab(s, mergedId, 0);
    expect(s.windows).toHaveLength(0);
  });
});
