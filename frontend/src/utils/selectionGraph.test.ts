import { describe, it, expect } from 'vitest';
import type { Node as RFNode, Edge as RFEdge } from '@xyflow/react';
import {
  isSelectionPresent,
  resolveConnectedElements,
  styleNodesForSelection,
  styleEdgesForSelection,
} from './selectionGraph';
import { FADE_OPACITY, EDGE_FADE_OPACITY } from '../utils/constants';
import { matchesSearch } from '../hooks/useSelectionHighlight';

function makeNode(id: string, type = 'containerNode', parentId?: string): RFNode {
  return { id, type, position: { x: 0, y: 0 }, data: {}, parentId } as RFNode;
}

function makeEdge(id: string, source: string, target: string): RFEdge {
  return { id, source, target } as RFEdge;
}

describe('resolveConnectedElements', () => {
  it('finds connected edges and nodes for a node selection', () => {
    const nodes = [makeNode('a'), makeNode('b'), makeNode('c')];
    const edges = [makeEdge('e1', 'a', 'b'), makeEdge('e2', 'b', 'c')];

    const result = resolveConnectedElements({ type: 'node', id: 'a' }, nodes, edges);

    expect(result.connectedNodeIds.has('a')).toBe(true);
    expect(result.connectedNodeIds.has('b')).toBe(true);
    expect(result.connectedNodeIds.has('c')).toBe(false);
    expect(result.connectedEdgeIds.has('e1')).toBe(true);
    expect(result.connectedEdgeIds.has('e2')).toBe(false);
  });

  it('finds both endpoints for an edge selection', () => {
    const nodes = [makeNode('a'), makeNode('b'), makeNode('c')];
    const edges = [makeEdge('e1', 'a', 'b'), makeEdge('e2', 'b', 'c')];

    const result = resolveConnectedElements({ type: 'edge', id: 'e1' }, nodes, edges);

    expect(result.connectedEdgeIds.has('e1')).toBe(true);
    expect(result.connectedEdgeIds.has('e2')).toBe(false);
    expect(result.connectedNodeIds.has('a')).toBe(true);
    expect(result.connectedNodeIds.has('b')).toBe(true);
  });

  it('highlights children and their edges for a group selection', () => {
    const group = makeNode('group1', 'networkGroup');
    const child1 = makeNode('c1', 'containerNode', 'group1');
    const child2 = makeNode('c2', 'containerNode', 'group1');
    const external = makeNode('ext', 'containerNode');
    const nodes = [group, child1, child2, external];
    const edges = [
      makeEdge('e1', 'c1', 'c2'),
      makeEdge('e2', 'c1', 'ext'),
    ];

    const result = resolveConnectedElements({ type: 'node', id: 'group1' }, nodes, edges);

    expect(result.highlightedGroupIds.has('group1')).toBe(true);
    expect(result.connectedNodeIds.has('c1')).toBe(true);
    expect(result.connectedNodeIds.has('c2')).toBe(true);
    expect(result.connectedNodeIds.has('ext')).toBe(true);
    expect(result.connectedEdgeIds.has('e1')).toBe(true);
    expect(result.connectedEdgeIds.has('e2')).toBe(true);
  });

  it('marks parent groups of highlighted nodes', () => {
    const group = makeNode('group1', 'networkGroup');
    const child = makeNode('c1', 'containerNode', 'group1');
    const standalone = makeNode('s1', 'containerNode');
    const nodes = [group, child, standalone];
    const edges = [makeEdge('e1', 's1', 'c1')];

    const result = resolveConnectedElements({ type: 'node', id: 's1' }, nodes, edges);

    expect(result.highlightedGroupIds.has('group1')).toBe(true);
  });

  it('node click: highlights network card but not its children', () => {
    const group = makeNode('net1', 'networkGroup');
    const child1 = makeNode('c1', 'containerNode', 'net1');
    const child2 = makeNode('c2', 'containerNode', 'net1');
    const external = makeNode('ext', 'containerNode');
    const nodes = [group, child1, child2, external];
    const edges = [makeEdge('e1', 'ext', 'net1')];

    const result = resolveConnectedElements({ type: 'node', id: 'ext' }, nodes, edges);

    expect(result.connectedNodeIds.has('ext')).toBe(true);
    expect(result.connectedNodeIds.has('net1')).toBe(true);
    expect(result.connectedNodeIds.has('c1')).toBe(false);
    expect(result.connectedNodeIds.has('c2')).toBe(false);
  });

  it('edge click: expands network group children when endpoint is a network', () => {
    const group = makeNode('net1', 'networkGroup');
    const child1 = makeNode('c1', 'containerNode', 'net1');
    const child2 = makeNode('c2', 'containerNode', 'net1');
    const external = makeNode('ext', 'containerNode');
    const nodes = [group, child1, child2, external];
    const edges = [makeEdge('e1', 'ext', 'net1')];

    const result = resolveConnectedElements({ type: 'edge', id: 'e1' }, nodes, edges);

    expect(result.connectedNodeIds.has('ext')).toBe(true);
    expect(result.connectedNodeIds.has('net1')).toBe(true);
    expect(result.connectedNodeIds.has('c1')).toBe(true);
    expect(result.connectedNodeIds.has('c2')).toBe(true);
  });

  it('handles edge selection for nonexistent edge', () => {
    const nodes = [makeNode('a')];
    const edges: RFEdge[] = [];

    const result = resolveConnectedElements({ type: 'edge', id: 'missing' }, nodes, edges);

    expect(result.connectedEdgeIds.size).toBe(0);
    expect(result.connectedNodeIds.size).toBe(0);
  });
});

describe('styleNodesForSelection', () => {
  it('sets opacity 1 for connected nodes and FADE_OPACITY for others', () => {
    const nodes = [makeNode('a'), makeNode('b')];
    const connected = new Set(['a']);
    const groups = new Set<string>();

    const styled = styleNodesForSelection(nodes, connected, groups);

    expect(styled[0].style?.opacity).toBe(1);
    expect(styled[1].style?.opacity).toBe(FADE_OPACITY);
  });

  it('uses highlightedGroupIds for networkGroup nodes', () => {
    const group = makeNode('g1', 'networkGroup');
    const nodes = [group];
    const connected = new Set<string>();
    const groups = new Set(['g1']);

    const styled = styleNodesForSelection(nodes, connected, groups);
    expect(styled[0].style?.opacity).toBe(1);
  });

  it('highlights networkGroup when it is a direct edge endpoint in connectedNodeIds', () => {
    const group = makeNode('net1', 'networkGroup');
    const nodes = [group];
    const connected = new Set(['net1']);
    const groups = new Set<string>();

    const styled = styleNodesForSelection(nodes, connected, groups);
    expect(styled[0].style?.opacity).toBe(1);
  });
});

describe('styleEdgesForSelection', () => {
  it('sets opacity 1 for connected edges and EDGE_FADE_OPACITY for others', () => {
    const edges = [makeEdge('e1', 'a', 'b'), makeEdge('e2', 'b', 'c')];
    const connected = new Set(['e1']);

    const styled = styleEdgesForSelection(edges, connected, false);

    expect(styled[0].style?.opacity).toBe(1);
    expect(styled[1].style?.opacity).toBe(EDGE_FADE_OPACITY);
  });
});

describe('per-node view (nested groups and service cards)', () => {
  function card(id: string, parentId: string, serviceId: string): RFNode {
    return { id, type: 'nodeServiceCard', position: { x: 0, y: 0 }, parentId, data: { serviceId } } as RFNode;
  }
  const nodes = [
    makeNode('rolegroup:manager', 'roleGroup'),
    makeNode('rolegroup:worker', 'roleGroup'),
    makeNode('swarmnode:m1', 'nodeGroup', 'rolegroup:manager'),
    makeNode('swarmnode:w1', 'nodeGroup', 'rolegroup:worker'),
    card('nodesvc:m1:web', 'swarmnode:m1', 'service:web'),
    card('nodesvc:m1:db', 'swarmnode:m1', 'service:db'),
    card('nodesvc:w1:web', 'swarmnode:w1', 'service:web'),
  ];

  it('lights every card of the selected service and all their ancestors', () => {
    const r = resolveConnectedElements({ type: 'node', id: 'nodesvc:m1:web' }, nodes, []);
    expect([...r.connectedNodeIds].sort()).toEqual(['nodesvc:m1:web', 'nodesvc:w1:web']);
    expect([...r.highlightedGroupIds].sort()).toEqual([
      'rolegroup:manager', 'rolegroup:worker', 'swarmnode:m1', 'swarmnode:w1',
    ]);
    const styled = styleNodesForSelection(nodes, r.connectedNodeIds, r.highlightedGroupIds);
    expect(styled.find((n) => n.id === 'nodesvc:m1:db')!.style?.opacity).toBe(FADE_OPACITY);
  });

  it('lights all descendants of a selected role group', () => {
    const r = resolveConnectedElements({ type: 'node', id: 'rolegroup:manager' }, nodes, []);
    expect([...r.connectedNodeIds].sort()).toEqual(['nodesvc:m1:db', 'nodesvc:m1:web', 'swarmnode:m1']);
    expect(r.highlightedGroupIds.has('swarmnode:m1')).toBe(true);
    expect(r.highlightedGroupIds.has('rolegroup:worker')).toBe(false);
  });

  it('lights the remote role group of a control link leaving the selected group', () => {
    const control = makeEdge('control:w1', 'rolegroup:manager', 'swarmnode:w1');
    const r = resolveConnectedElements({ type: 'node', id: 'rolegroup:manager' }, nodes, [control]);
    expect(r.connectedEdgeIds.has('control:w1')).toBe(true);
    expect(r.connectedNodeIds.has('swarmnode:w1')).toBe(true);
    expect(r.highlightedGroupIds.has('rolegroup:worker')).toBe(true);
  });

  it('lights every ancestor group of a remote endpoint nested in a box', () => {
    const link = makeEdge('overlay', 'nodesvc:m1:web', 'nodesvc:w1:web');
    const r = resolveConnectedElements({ type: 'node', id: 'rolegroup:manager' }, nodes, [link]);
    expect(r.highlightedGroupIds.has('swarmnode:w1')).toBe(true);
    expect(r.highlightedGroupIds.has('rolegroup:worker')).toBe(true);
  });

  it('keeps service cards lit when a search matches their service', () => {
    const matching = new Set(['service:web']);
    expect(matchesSearch(nodes[4], matching)).toBe(true);
    expect(matchesSearch(nodes[5], matching)).toBe(false);
    expect(matchesSearch(nodes[0], matching)).toBe(true); // groups stay lit
  });
});

describe('isSelectionPresent', () => {
  const nodes = [makeNode('a'), makeNode('b')];
  const edges = [makeEdge('e1', 'a', 'b')];

  it('finds a selected node or edge in the current graph', () => {
    expect(isSelectionPresent({ type: 'node', id: 'a' }, nodes, edges)).toBe(true);
    expect(isSelectionPresent({ type: 'edge', id: 'e1' }, nodes, edges)).toBe(true);
  });

  it('reports a selection left over from another topology', () => {
    expect(isSelectionPresent({ type: 'node', id: 'gone' }, nodes, edges)).toBe(false);
    expect(isSelectionPresent({ type: 'edge', id: 'gone' }, nodes, edges)).toBe(false);
  });

  it('does not match a node id against edges or the reverse', () => {
    expect(isSelectionPresent({ type: 'edge', id: 'a' }, nodes, edges)).toBe(false);
    expect(isSelectionPresent({ type: 'node', id: 'e1' }, nodes, edges)).toBe(false);
  });
});
