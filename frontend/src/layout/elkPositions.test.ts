import { describe, it, expect } from 'vitest';
import type { Node as RFNode } from '@xyflow/react';
import type { ElkNode } from 'elkjs/lib/elk.bundled';
import { applyElkPositions } from './elkPositions';

function makeNode(id: string, type = 'containerNode'): RFNode {
  return { id, type, position: { x: 0, y: 0 }, data: {}, style: {} } as RFNode;
}

describe('applyElkPositions', () => {
  it('positions a standalone top-level node', () => {
    const n = makeNode('a');
    const nodeMap = new Map([['a', n]]);
    const layout: ElkNode = {
      id: 'root',
      children: [{ id: 'a', x: 100, y: 200 }],
    };

    applyElkPositions(layout, nodeMap);

    expect(n.position).toEqual({ x: 100, y: 200 });
  });

  it('adds component wrapper offset to child positions', () => {
    const n = makeNode('a');
    const nodeMap = new Map([['a', n]]);
    const layout: ElkNode = {
      id: 'root',
      children: [{
        id: '__comp_a',
        x: 50,
        y: 30,
        children: [{ id: 'a', x: 10, y: 20 }],
      }],
    };

    applyElkPositions(layout, nodeMap);

    expect(n.position).toEqual({ x: 60, y: 50 });
  });

  it('sets width and height on networkGroup nodes', () => {
    const n = makeNode('net1', 'networkGroup');
    const nodeMap = new Map([['net1', n]]);
    const layout: ElkNode = {
      id: 'root',
      children: [{ id: 'net1', x: 0, y: 0, width: 300, height: 400 }],
    };

    applyElkPositions(layout, nodeMap);

    expect(n.style).toMatchObject({ width: 300, height: 400 });
  });

  it('does not set width/height on non-group nodes', () => {
    const n = makeNode('a');
    n.style = {};
    const nodeMap = new Map([['a', n]]);
    const layout: ElkNode = {
      id: 'root',
      children: [{ id: 'a', x: 10, y: 20, width: 180, height: 70 }],
    };

    applyElkPositions(layout, nodeMap);

    expect(n.style).toEqual({});
  });

  it('positions nested children relative to their parent', () => {
    const parent = makeNode('net1', 'networkGroup');
    const child = makeNode('c1');
    const nodeMap = new Map([['net1', parent], ['c1', child]]);
    const layout: ElkNode = {
      id: 'root',
      children: [{
        id: 'net1',
        x: 100, y: 50,
        width: 300, height: 200,
        children: [{ id: 'c1', x: 15, y: 35 }],
      }],
    };

    applyElkPositions(layout, nodeMap);

    expect(parent.position).toEqual({ x: 100, y: 50 });
    expect(child.position).toEqual({ x: 15, y: 35 });
  });

  it('defaults to 0 for missing x/y coordinates', () => {
    const n = makeNode('a');
    const nodeMap = new Map([['a', n]]);
    const layout: ElkNode = { id: 'root', children: [{ id: 'a' }] };

    applyElkPositions(layout, nodeMap);

    expect(n.position).toEqual({ x: 0, y: 0 });
  });

  it('skips nodes not present in nodeMap without throwing', () => {
    const nodeMap = new Map<string, RFNode>();
    const layout: ElkNode = {
      id: 'root',
      children: [{ id: 'missing', x: 100, y: 200 }],
    };

    expect(() => applyElkPositions(layout, nodeMap)).not.toThrow();
  });

  it('handles layout with no children', () => {
    const nodeMap = new Map<string, RFNode>();
    const layout: ElkNode = { id: 'root' };

    expect(() => applyElkPositions(layout, nodeMap)).not.toThrow();
  });

  it('handles wrapper with multiple nodes at different offsets', () => {
    const a = makeNode('a');
    const b = makeNode('b');
    const nodeMap = new Map([['a', a], ['b', b]]);
    const layout: ElkNode = {
      id: 'root',
      children: [{
        id: '__comp_x',
        x: 10,
        y: 20,
        children: [
          { id: 'a', x: 0, y: 0 },
          { id: 'b', x: 0, y: 80 },
        ],
      }],
    };

    applyElkPositions(layout, nodeMap);

    expect(a.position).toEqual({ x: 10, y: 20 });
    expect(b.position).toEqual({ x: 10, y: 100 });
  });

  it('skips missing children inside a parent node', () => {
    const parent = makeNode('net1', 'networkGroup');
    const nodeMap = new Map([['net1', parent]]);
    const layout: ElkNode = {
      id: 'root',
      children: [{
        id: 'net1', x: 0, y: 0, width: 300, height: 200,
        children: [{ id: 'missing_child', x: 10, y: 10 }],
      }],
    };

    expect(() => applyElkPositions(layout, nodeMap)).not.toThrow();
  });

  it('preserves existing style properties on networkGroup nodes', () => {
    const n = makeNode('net1', 'networkGroup');
    n.style = { opacity: 0.5, border: '1px solid red' };
    const nodeMap = new Map([['net1', n]]);
    const layout: ElkNode = {
      id: 'root',
      children: [{ id: 'net1', x: 0, y: 0, width: 300, height: 200 }],
    };

    applyElkPositions(layout, nodeMap);

    expect(n.style).toMatchObject({
      opacity: 0.5,
      border: '1px solid red',
      width: 300,
      height: 200,
    });
  });
});
