// @vitest-environment jsdom
import { describe, it, expect, afterEach } from 'vitest';
import { renderHook, act, cleanup } from '@testing-library/react';
import { useStackScope } from './useStackScope';

afterEach(() => {
  cleanup();
  window.history.replaceState(null, '', '/');
});

describe('useStackScope', () => {
  it('defaults to all stacks (null) without a ?stack= param', () => {
    const { result } = renderHook(() => useStackScope());
    expect(result.current.selectedStack).toBeNull();
  });

  it('reads the initial stack from the URL', () => {
    window.history.replaceState(null, '', '/?stack=shop');
    const { result } = renderHook(() => useStackScope());
    expect(result.current.selectedStack).toBe('shop');
  });

  it('writes the selection to ?stack= with replaceState, preserving other params', () => {
    window.history.replaceState(null, '', '/?foo=1#frag');
    const before = window.history.length;
    const { result } = renderHook(() => useStackScope());

    act(() => result.current.setSelectedStack('blog'));
    expect(result.current.selectedStack).toBe('blog');
    expect(window.location.search).toBe('?foo=1&stack=blog');
    expect(window.location.hash).toBe('#frag');
    expect(window.history.length).toBe(before);
  });

  it('clears the param when set back to all', () => {
    window.history.replaceState(null, '', '/?stack=shop');
    const { result } = renderHook(() => useStackScope());

    act(() => result.current.setSelectedStack(null));
    expect(result.current.selectedStack).toBeNull();
    expect(window.location.search).toBe('');
  });
});
