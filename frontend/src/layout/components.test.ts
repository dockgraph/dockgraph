import { describe, it, expect } from 'vitest';
import type { Edge as RFEdge } from '@xyflow/react';
import { findComponents } from './components';

function makeEdge(id: string, source: string, target: string): RFEdge {
  return { id, source, target } as RFEdge;
}

describe('findComponents', () => {
  it('returns a single component when all nodes are connected', () => {
    const topIds = ['a', 'b', 'c'];
    const edges = [makeEdge('e1', 'a', 'b'), makeEdge('e2', 'b', 'c')];
    const childToParent = new Map<string, string>();

    const result = findComponents(topIds, edges, childToParent);

    expect(result).toHaveLength(1);
    expect(result[0].sort()).toEqual(['a', 'b', 'c']);
  });

  it('returns separate components for disconnected nodes', () => {
    const topIds = ['a', 'b', 'c'];
    const edges = [makeEdge('e1', 'a', 'b')];
    const childToParent = new Map<string, string>();

    const result = findComponents(topIds, edges, childToParent);

    expect(result).toHaveLength(2);
    const sizes = result.map((c) => c.length).sort();
    expect(sizes).toEqual([1, 2]);
  });

  it('returns each node as its own component when there are no edges', () => {
    const topIds = ['a', 'b', 'c'];
    const result = findComponents(topIds, [], new Map());

    expect(result).toHaveLength(3);
  });

  it('returns empty for empty input', () => {
    expect(findComponents([], [], new Map())).toEqual([]);
  });

  it('resolves edges through child-to-parent mapping', () => {
    // child1 belongs to groupA, child2 belongs to groupB
    const topIds = ['groupA', 'groupB'];
    const edges = [makeEdge('e1', 'child1', 'child2')];
    const childToParent = new Map([
      ['child1', 'groupA'],
      ['child2', 'groupB'],
    ]);

    const result = findComponents(topIds, edges, childToParent);

    // Both groups should be in the same component
    expect(result).toHaveLength(1);
    expect(result[0].sort()).toEqual(['groupA', 'groupB']);
  });

  it('ignores self-edges (same parent)', () => {
    const topIds = ['groupA', 'groupB'];
    const edges = [makeEdge('e1', 'child1', 'child2')];
    const childToParent = new Map([
      ['child1', 'groupA'],
      ['child2', 'groupA'],
    ]);

    const result = findComponents(topIds, edges, childToParent);

    // Self-edge doesn't connect two groups
    expect(result).toHaveLength(2);
  });
});
