import { describe, it, expect } from 'vitest';
import type { Node as RFNode, Edge as RFEdge } from '@xyflow/react';
import type { ElkNode, ElkExtendedEdge } from 'elkjs/lib/elk.bundled';
import { classifyNodes, buildElkChildren, wrapComponents } from './elkGraph';
import { CONTAINER_NODE_HEIGHT, VOLUME_NODE_HEIGHT } from '../utils/constants';

function makeNode(id: string, type = 'containerNode', parentId?: string): RFNode {
  return { id, type, position: { x: 0, y: 0 }, data: {}, parentId } as RFNode;
}

function makeEdge(id: string, source: string, target: string): RFEdge {
  return { id, source, target } as RFEdge;
}

// --- classifyNodes ---

describe('classifyNodes', () => {
  it('separates groups, children, and free-standing nodes', () => {
    const nodes = [
      makeNode('net1', 'networkGroup'),
      makeNode('c1', 'containerNode', 'net1'),
      makeNode('c2', 'containerNode', 'net1'),
      makeNode('free1'),
      makeNode('vol1', 'volumeNode'),
    ];

    const { groups, children, freeNodes, childToParent } = classifyNodes(nodes);

    expect(groups.map((n) => n.id)).toEqual(['net1']);
    expect(children.map((n) => n.id)).toEqual(['c1', 'c2']);
    expect(freeNodes.map((n) => n.id)).toEqual(['free1', 'vol1']);
    expect(childToParent.get('c1')).toBe('net1');
    expect(childToParent.get('c2')).toBe('net1');
  });

  it('returns empty collections for empty input', () => {
    const { groups, children, freeNodes, childToParent } = classifyNodes([]);

    expect(groups).toHaveLength(0);
    expect(children).toHaveLength(0);
    expect(freeNodes).toHaveLength(0);
    expect(childToParent.size).toBe(0);
  });

  it('puts all nodes into freeNodes when there are no groups', () => {
    const nodes = [makeNode('a'), makeNode('b'), makeNode('c')];
    const { groups, freeNodes } = classifyNodes(nodes);

    expect(groups).toHaveLength(0);
    expect(freeNodes).toHaveLength(3);
  });

  it('volume nodes without parents are free nodes, not groups', () => {
    const nodes = [makeNode('v1', 'volumeNode')];
    const { groups, freeNodes } = classifyNodes(nodes);

    expect(groups).toHaveLength(0);
    expect(freeNodes).toHaveLength(1);
  });

  it('child nodes with parentId appear in both children and childToParent', () => {
    const nodes = [
      makeNode('net1', 'networkGroup'),
      makeNode('net2', 'networkGroup'),
      makeNode('c1', 'containerNode', 'net1'),
      makeNode('c2', 'containerNode', 'net2'),
    ];

    const { children, childToParent } = classifyNodes(nodes);

    expect(children).toHaveLength(2);
    expect(childToParent.size).toBe(2);
    expect(childToParent.get('c1')).toBe('net1');
    expect(childToParent.get('c2')).toBe('net2');
  });
});

// --- buildElkChildren ---

describe('buildElkChildren', () => {
  it('creates ELK leaf nodes with correct dimensions', () => {
    const a = makeNode('a');
    const b = makeNode('b', 'volumeNode');
    const nodeMap = new Map([['a', a], ['b', b]]);

    const { elkChildren } = buildElkChildren(
      [['a', 'b']], nodeMap, [], [], new Map(), 180,
    );

    expect(elkChildren).toHaveLength(2);
    expect(elkChildren[0]).toMatchObject({ id: 'a', width: 180, height: CONTAINER_NODE_HEIGHT });
    expect(elkChildren[1]).toMatchObject({ id: 'b', width: 180, height: VOLUME_NODE_HEIGHT });
  });

  it('builds group nodes with their children', () => {
    const net = makeNode('net1', 'networkGroup');
    const c1 = makeNode('c1', 'containerNode', 'net1');
    const c2 = makeNode('c2', 'containerNode', 'net1');
    const nodeMap = new Map([['net1', net], ['c1', c1], ['c2', c2]]);
    const childToParent = new Map([['c1', 'net1'], ['c2', 'net1']]);

    const { elkChildren } = buildElkChildren(
      [['net1']], nodeMap, [c1, c2], [], childToParent, 200,
    );

    expect(elkChildren).toHaveLength(1);
    expect(elkChildren[0].children).toHaveLength(2);
    expect(elkChildren[0].children![0]).toMatchObject({ id: 'c1', width: 200, height: CONTAINER_NODE_HEIGHT });
  });

  it('inserts a placeholder for empty network groups', () => {
    const net = makeNode('net1', 'networkGroup');
    const nodeMap = new Map([['net1', net]]);

    const { elkChildren } = buildElkChildren(
      [['net1']], nodeMap, [], [], new Map(), 180,
    );

    expect(elkChildren[0].children).toHaveLength(1);
    expect(elkChildren[0].children![0].id).toBe('net1__placeholder');
    expect(elkChildren[0].children![0].height).toBe(1);
  });

  it('attaches same-group edges to the group, not to root', () => {
    const net = makeNode('net1', 'networkGroup');
    const c1 = makeNode('c1', 'containerNode', 'net1');
    const c2 = makeNode('c2', 'containerNode', 'net1');
    const nodeMap = new Map([['net1', net], ['c1', c1], ['c2', c2]]);
    const childToParent = new Map([['c1', 'net1'], ['c2', 'net1']]);
    const edges = [makeEdge('e1', 'c1', 'c2')];

    const { elkChildren, rootEdges } = buildElkChildren(
      [['net1']], nodeMap, [c1, c2], edges, childToParent, 180,
    );

    expect(rootEdges).toHaveLength(0);
    const groupNode = elkChildren[0] as { edges?: unknown[] };
    expect(groupNode.edges).toHaveLength(1);
  });

  it('puts cross-group edges at root level', () => {
    const net = makeNode('net1', 'networkGroup');
    const c1 = makeNode('c1', 'containerNode', 'net1');
    const free = makeNode('free1');
    const nodeMap = new Map([['net1', net], ['c1', c1], ['free1', free]]);
    const childToParent = new Map([['c1', 'net1']]);
    const edges = [makeEdge('e1', 'c1', 'free1')];

    const { rootEdges } = buildElkChildren(
      [['net1', 'free1']], nodeMap, [c1], edges, childToParent, 180,
    );

    expect(rootEdges).toHaveLength(1);
    expect(rootEdges[0].id).toBe('e1');
  });

  it('ignores edges whose endpoints are outside the component', () => {
    const a = makeNode('a');
    const b = makeNode('b');
    const nodeMap = new Map([['a', a], ['b', b]]);
    const edges = [
      makeEdge('e1', 'a', 'b'),
      makeEdge('e2', 'a', 'outside'),
    ];

    const { rootEdges } = buildElkChildren(
      [['a', 'b']], nodeMap, [], edges, new Map(), 180,
    );

    expect(rootEdges).toHaveLength(1);
    expect(rootEdges[0].id).toBe('e1');
  });

  it('skips nodes not found in nodeMap', () => {
    const a = makeNode('a');
    const nodeMap = new Map([['a', a]]);

    const { elkChildren } = buildElkChildren(
      [['a', 'ghost']], nodeMap, [], [], new Map(), 180,
    );

    expect(elkChildren).toHaveLength(1);
    expect(elkChildren[0].id).toBe('a');
  });

  it('handles multiple components independently', () => {
    const a = makeNode('a');
    const b = makeNode('b');
    const c = makeNode('c');
    const nodeMap = new Map([['a', a], ['b', b], ['c', c]]);
    const edges = [makeEdge('e1', 'a', 'b')];

    const { elkChildren, rootEdges } = buildElkChildren(
      [['a', 'b'], ['c']], nodeMap, [], edges, new Map(), 180,
    );

    expect(elkChildren).toHaveLength(3);
    expect(rootEdges).toHaveLength(1);
  });
});

// --- wrapComponents ---

describe('wrapComponents', () => {
  it('wraps multi-node components with edges into a wrapper', () => {
    const elkChildren = [
      { id: 'a', width: 180, height: 70 },
      { id: 'b', width: 180, height: 70 },
    ];
    const rootEdges = [{ id: 'e1', sources: ['a'], targets: ['b'] }];

    const { wrappedChildren } = wrapComponents(
      [['a', 'b']], elkChildren as ElkNode[], rootEdges as ElkExtendedEdge[], new Map(),
    );

    expect(wrappedChildren).toHaveLength(1);
    expect(wrappedChildren[0].id).toBe('__comp_a');
    expect(wrappedChildren[0].children).toHaveLength(2);
    expect(wrappedChildren[0].edges).toHaveLength(1);
  });

  it('does not wrap a single-node component without edges', () => {
    const elkChildren = [{ id: 'a', width: 180, height: 70 }];

    const { wrappedChildren } = wrapComponents(
      [['a']], elkChildren as ElkNode[], [], new Map(),
    );

    expect(wrappedChildren).toHaveLength(1);
    expect(wrappedChildren[0].id).toBe('a');
  });

  it('wraps multi-node component even without root edges', () => {
    const elkChildren = [
      { id: 'a', width: 180, height: 70 },
      { id: 'b', width: 180, height: 70 },
    ];

    const { wrappedChildren } = wrapComponents(
      [['a', 'b']], elkChildren as ElkNode[], [], new Map(),
    );

    // Two nodes, zero edges — still wraps because compChildren.length > 1
    expect(wrappedChildren[0].id).toBe('__comp_a');
  });

  it('resolves edges through childToParent for correct component assignment', () => {
    const elkChildren = [
      { id: 'net1', width: 300, height: 200 },
      { id: 'free1', width: 180, height: 70 },
    ];
    const rootEdges = [{ id: 'e1', sources: ['c1'], targets: ['free1'] }];
    const childToParent = new Map([['c1', 'net1']]);

    const { wrappedChildren } = wrapComponents(
      [['net1', 'free1']], elkChildren as ElkNode[], rootEdges as ElkExtendedEdge[], childToParent,
    );

    expect(wrappedChildren[0].id).toBe('__comp_net1');
    expect(wrappedChildren[0].edges).toHaveLength(1);
  });

  it('handles multiple independent components', () => {
    const elkChildren = [
      { id: 'a', width: 180, height: 70 },
      { id: 'b', width: 180, height: 70 },
      { id: 'c', width: 180, height: 70 },
    ];
    const rootEdges = [{ id: 'e1', sources: ['a'], targets: ['b'] }];

    const { wrappedChildren } = wrapComponents(
      [['a', 'b'], ['c']], elkChildren as ElkNode[], rootEdges as ElkExtendedEdge[], new Map(),
    );

    expect(wrappedChildren).toHaveLength(2);
    expect(wrappedChildren[0].id).toBe('__comp_a');
    expect(wrappedChildren[1].id).toBe('c'); // single, no edges → unwrapped
  });
});
