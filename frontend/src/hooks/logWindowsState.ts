import {
  WINDOW_DEFAULT_W,
  WINDOW_DEFAULT_H,
  WINDOW_MIN_W,
  WINDOW_MIN_H,
  WINDOW_CASCADE,
} from '../utils/constants';

/** One container's logs within a window. */
export interface LogTab {
  containerId: string;
  title: string;
  search: string;
}

/** A single floating log window. Coordinates are viewport-relative (fixed). */
export interface LogWindowState {
  id: string;
  tabs: LogTab[];
  activeTab: number;
  x: number;
  y: number;
  w: number;
  h: number;
  z: number;
  minimized: boolean;
}

/** The whole windows subsystem state. Kept serializable and pure. */
export interface WindowsState {
  windows: LogWindowState[];
  nextZ: number;
  nextId: number;
}

/** Viewport metrics used for clamping window positions. */
export interface Viewport {
  width: number;
  height: number;
  headerHeight: number;
  dockHeight: number;
}

export const initialWindowsState: WindowsState = { windows: [], nextZ: 1, nextId: 1 };

/**
 * Keeps a window on screen: fully within the horizontal bounds, its title bar
 * below the header, and never dragged past the dock (a 40px margin keeps the
 * title bar reachable).
 */
export function clampPosition(
  x: number,
  y: number,
  w: number,
  vp: Viewport,
): { x: number; y: number } {
  const maxX = Math.max(0, vp.width - w);
  const minY = vp.headerHeight;
  const maxY = Math.max(minY, vp.height - vp.dockHeight - 40);
  return {
    x: Math.min(Math.max(0, x), maxX),
    y: Math.min(Math.max(minY, y), maxY),
  };
}

/** Finds the window holding a tab for the given container, if any. */
function findContainerWindow(s: WindowsState, containerId: string): LogWindowState | undefined {
  return s.windows.find((w) => w.tabs.some((t) => t.containerId === containerId));
}

export function focusWindow(s: WindowsState, id: string): WindowsState {
  const z = s.nextZ;
  return {
    ...s,
    nextZ: z + 1,
    windows: s.windows.map((w) => (w.id === id ? { ...w, z } : w)),
  };
}

export function openLogs(
  s: WindowsState,
  containerId: string,
  title: string,
  vp: Viewport,
): WindowsState {
  // Focus-existing: never duplicate a container's stream.
  const existing = findContainerWindow(s, containerId);
  if (existing) {
    const tabIndex = existing.tabs.findIndex((t) => t.containerId === containerId);
    const z = s.nextZ;
    return {
      ...s,
      nextZ: z + 1,
      windows: s.windows.map((w) =>
        w.id === existing.id ? { ...w, z, minimized: false, activeTab: tabIndex } : w,
      ),
    };
  }

  const count = s.windows.length;
  const baseX = 80 + count * WINDOW_CASCADE;
  const baseY = vp.headerHeight + 24 + count * WINDOW_CASCADE;
  const { x, y } = clampPosition(baseX, baseY, WINDOW_DEFAULT_W, vp);
  const win: LogWindowState = {
    id: `lw-${s.nextId}`,
    tabs: [{ containerId, title, search: '' }],
    activeTab: 0,
    x,
    y,
    w: WINDOW_DEFAULT_W,
    h: WINDOW_DEFAULT_H,
    z: s.nextZ,
    minimized: false,
  };
  return {
    windows: [...s.windows, win],
    nextZ: s.nextZ + 1,
    nextId: s.nextId + 1,
  };
}

export function closeWindow(s: WindowsState, id: string): WindowsState {
  return { ...s, windows: s.windows.filter((w) => w.id !== id) };
}

// Min size kept here for re-use by resize transitions.
export const MIN_SIZE = { w: WINDOW_MIN_W, h: WINDOW_MIN_H };

export function moveWindow(s: WindowsState, id: string, x: number, y: number, vp: Viewport): WindowsState {
  return {
    ...s,
    windows: s.windows.map((w) => {
      if (w.id !== id) return w;
      const p = clampPosition(x, y, w.w, vp);
      return { ...w, x: p.x, y: p.y };
    }),
  };
}

export function resizeWindow(s: WindowsState, id: string, w: number, h: number): WindowsState {
  return {
    ...s,
    windows: s.windows.map((win) =>
      win.id === id
        ? { ...win, w: Math.max(MIN_SIZE.w, Math.round(w)), h: Math.max(MIN_SIZE.h, Math.round(h)) }
        : win,
    ),
  };
}

export function minimizeWindow(s: WindowsState, id: string): WindowsState {
  return { ...s, windows: s.windows.map((w) => (w.id === id ? { ...w, minimized: true } : w)) };
}

export function restoreWindow(s: WindowsState, id: string): WindowsState {
  const z = s.nextZ;
  return {
    ...s,
    nextZ: z + 1,
    windows: s.windows.map((w) => (w.id === id ? { ...w, minimized: false, z } : w)),
  };
}

export function setActiveTab(s: WindowsState, id: string, idx: number): WindowsState {
  return {
    ...s,
    windows: s.windows.map((w) => {
      if (w.id !== id) return w;
      const clamped = Math.min(Math.max(0, idx), w.tabs.length - 1);
      return { ...w, activeTab: clamped };
    }),
  };
}

export function setSearch(s: WindowsState, id: string, tabIndex: number, query: string): WindowsState {
  return {
    ...s,
    windows: s.windows.map((w) => {
      if (w.id !== id) return w;
      return { ...w, tabs: w.tabs.map((t, i) => (i === tabIndex ? { ...t, search: query } : t)) };
    }),
  };
}

/** Appends all of source's tabs into target, activating the first moved tab, and removes source. */
export function mergeWindows(s: WindowsState, sourceId: string, targetId: string): WindowsState {
  if (sourceId === targetId) return s;
  const source = s.windows.find((w) => w.id === sourceId);
  const target = s.windows.find((w) => w.id === targetId);
  if (!source || !target) return s;

  const z = s.nextZ;
  const mergedTabs = [...target.tabs, ...source.tabs];
  return {
    ...s,
    nextZ: z + 1,
    windows: s.windows
      .filter((w) => w.id !== sourceId)
      .map((w) =>
        w.id === targetId
          ? { ...w, tabs: mergedTabs, activeTab: target.tabs.length, z, minimized: false }
          : w,
      ),
  };
}

/** Pops a tab out of a multi-tab window into a fresh window at (x, y). No-op for single-tab windows. */
export function detachTab(s: WindowsState, id: string, tabIndex: number, x: number, y: number): WindowsState {
  const win = s.windows.find((w) => w.id === id);
  if (!win || win.tabs.length <= 1) return s;
  const tab = win.tabs[tabIndex];
  if (!tab) return s;

  const remaining = win.tabs.filter((_, i) => i !== tabIndex);
  const newWin: LogWindowState = {
    id: `lw-${s.nextId}`,
    tabs: [tab],
    activeTab: 0,
    x,
    y,
    w: win.w,
    h: win.h,
    z: s.nextZ,
    minimized: false,
  };
  return {
    nextZ: s.nextZ + 1,
    nextId: s.nextId + 1,
    windows: s.windows
      .map((w) =>
        w.id === id
          ? { ...w, tabs: remaining, activeTab: Math.min(w.activeTab, remaining.length - 1) }
          : w,
      )
      .concat(newWin),
  };
}

/** Removes a single tab; if it was the last tab, the window is removed entirely. */
export function closeTab(s: WindowsState, id: string, tabIndex: number): WindowsState {
  const win = s.windows.find((w) => w.id === id);
  if (!win) return s;
  if (win.tabs.length <= 1) return closeWindow(s, id);
  const remaining = win.tabs.filter((_, i) => i !== tabIndex);
  return {
    ...s,
    windows: s.windows.map((w) =>
      w.id === id ? { ...w, tabs: remaining, activeTab: Math.min(w.activeTab, remaining.length - 1) } : w,
    ),
  };
}
