// @vitest-environment jsdom
import { describe, it, expect, beforeEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { ThemeProvider, useTheme } from './theme';
import type { ReactNode } from 'react';

function wrapper({ children }: { children: ReactNode }) {
  return <ThemeProvider>{children}</ThemeProvider>;
}

beforeEach(() => {
  localStorage.clear();
});

describe('ThemeProvider', () => {
  it('defaults to dark theme', () => {
    const { result } = renderHook(() => useTheme(), { wrapper });
    expect(result.current.theme.mode).toBe('dark');
  });

  it('toggles between dark and light', () => {
    const { result } = renderHook(() => useTheme(), { wrapper });

    act(() => result.current.toggle());
    expect(result.current.theme.mode).toBe('light');

    act(() => result.current.toggle());
    expect(result.current.theme.mode).toBe('dark');
  });

  it('persists theme to localStorage on toggle', () => {
    const { result } = renderHook(() => useTheme(), { wrapper });

    act(() => result.current.toggle());
    expect(localStorage.getItem('dockgraph-theme')).toBe('light');

    act(() => result.current.toggle());
    expect(localStorage.getItem('dockgraph-theme')).toBe('dark');
  });

  it('reads initial mode from localStorage', () => {
    localStorage.setItem('dockgraph-theme', 'light');

    const { result } = renderHook(() => useTheme(), { wrapper });
    expect(result.current.theme.mode).toBe('light');
  });

  it('falls back to dark for invalid localStorage value', () => {
    localStorage.setItem('dockgraph-theme', 'invalid');

    const { result } = renderHook(() => useTheme(), { wrapper });
    expect(result.current.theme.mode).toBe('dark');
  });

  it('dark theme has all expected color properties', () => {
    const { result } = renderHook(() => useTheme(), { wrapper });
    const t = result.current.theme;

    expect(t.canvasBg).toMatch(/^#/);
    expect(t.nodeBg).toMatch(/^#/);
    expect(t.nodeText).toMatch(/^#/);
    expect(t.edgeStroke).toMatch(/^#/);
    expect(t.panelBg).toMatch(/^#/);
    expect(t.minimapBg).toMatch(/^#/);
    expect(t.groupBgAlpha).toBeDefined();
  });

  it('light theme has distinct colors from dark', () => {
    const { result } = renderHook(() => useTheme(), { wrapper });

    const darkBg = result.current.theme.canvasBg;
    act(() => result.current.toggle());
    const lightBg = result.current.theme.canvasBg;

    expect(darkBg).not.toBe(lightBg);
  });
});
