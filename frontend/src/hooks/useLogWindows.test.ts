// @vitest-environment jsdom
import { describe, it, expect } from 'vitest';
import { act, renderHook } from '@testing-library/react';
import { useLogWindows } from './useLogWindows';

describe('useLogWindows', () => {
  it('opens, focuses-existing, and closes windows', () => {
    const { result } = renderHook(() => useLogWindows());

    act(() => result.current.openLogs('nginx', 'nginx-1'));
    expect(result.current.windows).toHaveLength(1);

    // focus-existing: no duplicate
    act(() => result.current.openLogs('nginx', 'nginx-1'));
    expect(result.current.windows).toHaveLength(1);

    const id = result.current.windows[0].id;
    act(() => result.current.closeWindow(id));
    expect(result.current.windows).toHaveLength(0);
  });

  it('flags a flashed window briefly when re-opening an existing container', () => {
    const { result } = renderHook(() => useLogWindows());
    act(() => result.current.openLogs('a', 'a'));
    const id = result.current.windows[0].id;
    act(() => result.current.openLogs('a', 'a'));
    expect(result.current.flashId).toBe(id);
  });
});
