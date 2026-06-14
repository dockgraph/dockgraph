import { useCallback, useEffect, useRef, useState } from 'react';
import { HEADER_HEIGHT, DOCK_HEIGHT } from '../utils/constants';
import * as state from './logWindowsState';
import type { Viewport, WindowsState } from './logWindowsState';

/** Reads the current viewport metrics. Falls back to sane values outside a browser. */
function readViewport(): Viewport {
  return {
    width: typeof window !== 'undefined' ? window.innerWidth : 1280,
    height: typeof window !== 'undefined' ? window.innerHeight : 800,
    headerHeight: HEADER_HEIGHT,
    dockHeight: DOCK_HEIGHT,
  };
}

/**
 * Floating log window controller. Holds the pure WindowsState and exposes
 * bound, viewport-aware actions plus a transient `flashId` used to pulse a
 * window when the user re-opens an already-open container.
 */
export function useLogWindows() {
  const [s, setS] = useState<WindowsState>(state.initialWindowsState);
  const [flashId, setFlashId] = useState<string | null>(null);
  const vpRef = useRef<Viewport>(readViewport());

  useEffect(() => {
    const onResize = () => {
      vpRef.current = readViewport();
    };
    window.addEventListener('resize', onResize);
    return () => window.removeEventListener('resize', onResize);
  }, []);

  // Clear the flash highlight shortly after it is set.
  useEffect(() => {
    if (!flashId) return;
    const t = setTimeout(() => setFlashId(null), 700);
    return () => clearTimeout(t);
  }, [flashId]);

  const openLogs = useCallback((containerId: string, title: string) => {
    setS((prev) => {
      const alreadyOpen = prev.windows.some((w) => w.tabs.some((t) => t.containerId === containerId));
      const next = state.openLogs(prev, containerId, title, vpRef.current);
      if (alreadyOpen) {
        const win = next.windows.find((w) => w.tabs.some((t) => t.containerId === containerId));
        if (win) setFlashId(win.id);
      }
      return next;
    });
  }, []);

  const closeWindow = useCallback((id: string) => setS((p) => state.closeWindow(p, id)), []);
  const closeTab = useCallback((id: string, i: number) => setS((p) => state.closeTab(p, id, i)), []);
  const focusWindow = useCallback((id: string) => setS((p) => state.focusWindow(p, id)), []);
  const move = useCallback((id: string, x: number, y: number) => setS((p) => state.moveWindow(p, id, x, y, vpRef.current)), []);
  const resize = useCallback((id: string, w: number, h: number) => setS((p) => state.resizeWindow(p, id, w, h)), []);
  const minimize = useCallback((id: string) => setS((p) => state.minimizeWindow(p, id)), []);
  const restore = useCallback((id: string) => setS((p) => state.restoreWindow(p, id)), []);
  const setActiveTab = useCallback((id: string, i: number) => setS((p) => state.setActiveTab(p, id, i)), []);
  const setSearch = useCallback((id: string, i: number, q: string) => setS((p) => state.setSearch(p, id, i, q)), []);
  const merge = useCallback((sourceId: string, targetId: string) => setS((p) => state.mergeWindows(p, sourceId, targetId)), []);
  const detachTab = useCallback((id: string, i: number, x: number, y: number) => setS((p) => state.detachTab(p, id, i, x, y)), []);

  return {
    windows: s.windows,
    flashId,
    openLogs,
    closeWindow,
    closeTab,
    focusWindow,
    move,
    resize,
    minimize,
    restore,
    setActiveTab,
    setSearch,
    merge,
    detachTab,
  };
}

export type LogWindowsController = ReturnType<typeof useLogWindows>;
