// @vitest-environment jsdom
import { describe, it, expect, afterEach } from 'vitest';
import { renderHook, act, cleanup } from '@testing-library/react';
import { useGroupBy } from './useGroupBy';

afterEach(() => {
  cleanup();
  window.history.replaceState(null, '', '/');
});

describe('useGroupBy', () => {
  it('defaults to network without a ?group= param', () => {
    const { result } = renderHook(() => useGroupBy());
    expect(result.current.groupBy).toBe('network');
  });

  it('reads node mode from the URL', () => {
    window.history.replaceState(null, '', '/?group=node');
    const { result } = renderHook(() => useGroupBy());
    expect(result.current.groupBy).toBe('node');
  });

  it('ignores unknown values', () => {
    window.history.replaceState(null, '', '/?group=bogus');
    const { result } = renderHook(() => useGroupBy());
    expect(result.current.groupBy).toBe('network');
  });

  it('writes ?group=node with replaceState, preserving other params', () => {
    window.history.replaceState(null, '', '/?stack=shop#frag');
    const before = window.history.length;
    const { result } = renderHook(() => useGroupBy());

    act(() => result.current.setGroupBy('node'));
    expect(result.current.groupBy).toBe('node');
    expect(window.location.search).toBe('?stack=shop&group=node');
    expect(window.location.hash).toBe('#frag');
    expect(window.history.length).toBe(before);
  });

  it('drops the param when switching back to network', () => {
    window.history.replaceState(null, '', '/?group=node&stack=shop');
    const { result } = renderHook(() => useGroupBy());

    act(() => result.current.setGroupBy('network'));
    expect(result.current.groupBy).toBe('network');
    expect(window.location.search).toBe('?stack=shop');
  });
});
