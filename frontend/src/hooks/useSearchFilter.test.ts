// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { act, cleanup, renderHook } from '@testing-library/react';
import { useSearchFilter } from './useSearchFilter';
import type { DGNode } from '../types';

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

const nodes: DGNode[] = [
  // Swarm nodes carry their stack in `stack`, not in the payload labels.
  { id: 'network:mesh', type: 'network', name: 'mesh', stack: 'shop' },
  { id: 'container:api-1', type: 'container', name: 'api-1', labels: { 'com.docker.compose.project': 'blog' } },
  { id: 'container:lone', type: 'container', name: 'lone' },
];

function search(query: string) {
  vi.useFakeTimers();
  const { result } = renderHook(() => useSearchFilter(nodes));
  act(() => result.current.setQuery(query));
  act(() => { vi.runAllTimers(); });
  return result.current.matchingNodeIds;
}

describe('useSearchFilter', () => {
  it('matches a node by its stack', () => {
    expect([...search('shop')!]).toEqual(['network:mesh']);
  });

  it('matches a node by a payload label value', () => {
    expect([...search('blog')!]).toEqual(['container:api-1']);
  });
});
