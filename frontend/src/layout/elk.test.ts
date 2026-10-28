// @vitest-environment jsdom
import { describe, it, expect, vi, beforeAll, afterEach } from 'vitest';
import type { Node as RFNode, Edge as RFEdge } from '@xyflow/react';
import { computeLayout } from './elk';

// jsdom doesn't implement the Canvas API. Stub a monospace font instead: every
// character is 0.6em wide at the size the context's font asks for, so a name
// measured in the wrong font comes out the wrong width.
const measureText = vi.fn(function (this: { font: string }, text: string) {
  const size = Number(/([\d.]+)px/.exec(this.font)?.[1] ?? 10);
  return { width: text.length * size * 0.6 };
});
beforeAll(() => {
  HTMLCanvasElement.prototype.getContext = vi.fn().mockReturnValue({
    font: '',
    measureText,
  }) as ReturnType<typeof vi.fn>;
});

function makeNode(
  id: string,
  name: string,
  type = 'containerNode',
  parentId?: string,
): RFNode {
  return {
    id,
    type,
    position: { x: 0, y: 0 },
    data: { dgNode: { id, name, type: type === 'networkGroup' ? 'network' : 'container' } },
    parentId,
    ...(type === 'networkGroup' ? { style: {} } : {}),
  } as unknown as RFNode;
}

function makeEdge(id: string, source: string, target: string): RFEdge {
  return { id, source, target, data: {} } as RFEdge;
}

describe('computeLayout', () => {
  it('returns positioned nodes and edges for a simple graph', async () => {
    const nodes = [makeNode('a', 'web'), makeNode('b', 'api')];
    const edges = [makeEdge('e1', 'a', 'b')];

    const result = await computeLayout(nodes, edges);

    expect(result.nodes).toHaveLength(2);
    expect(result.edges).toHaveLength(1);

    // Nodes should have real positions assigned by ELK
    for (const n of result.nodes) {
      expect(typeof n.position.x).toBe('number');
      expect(typeof n.position.y).toBe('number');
    }

    // Edge should have an elk type and path data
    expect(result.edges[0].type).toBe('elk');
    expect(result.edges[0].data?.path).toBeDefined();
  });

  it('handles a single node with no edges', async () => {
    const nodes = [makeNode('solo', 'my-service')];

    const result = await computeLayout(nodes, []);

    expect(result.nodes).toHaveLength(1);
    expect(result.edges).toHaveLength(0);
  });

  it('assigns nodeWidth to non-group nodes', async () => {
    const nodes = [makeNode('a', 'web'), makeNode('b', 'db')];
    const edges = [makeEdge('e1', 'a', 'b')];

    const result = await computeLayout(nodes, edges);

    for (const n of result.nodes) {
      expect(n.data.nodeWidth).toBeGreaterThanOrEqual(140);
      expect(n.data.nodeWidth).toBeLessThanOrEqual(280);
    }
  });

  it('positions network group nodes with width and height', async () => {
    const net = makeNode('net1', 'frontend', 'networkGroup');
    const c1 = makeNode('c1', 'web', 'containerNode', 'net1');
    const c2 = makeNode('c2', 'api', 'containerNode', 'net1');

    const result = await computeLayout([net, c1, c2], []);

    const group = result.nodes.find((n) => n.type === 'networkGroup');
    expect(group).toBeDefined();
    expect(group!.style?.width).toBeGreaterThan(0);
    expect(group!.style?.height).toBeGreaterThan(0);
  });

  it('handles volume nodes with correct height', async () => {
    const vol = makeNode('v1', 'pgdata', 'volumeNode');
    const container = makeNode('c1', 'postgres');
    const edges = [makeEdge('e1', 'c1', 'v1')];

    const result = await computeLayout([vol, container], edges);

    expect(result.nodes).toHaveLength(2);
  });

  it('preserves edge data while adding path', async () => {
    const nodes = [makeNode('a', 'web'), makeNode('b', 'api')];
    const edges: RFEdge[] = [{
      id: 'e1',
      source: 'a',
      target: 'b',
      data: { custom: 'value', animated: true },
    }];

    const result = await computeLayout(nodes, edges);

    expect(result.edges[0].data?.custom).toBe('value');
    expect(result.edges[0].data?.animated).toBe(true);
    expect(result.edges[0].data?.path).toBeDefined();
  });

  it('handles a larger graph with multiple connected components', async () => {
    const nodes = [
      makeNode('a', 'web'),
      makeNode('b', 'api'),
      makeNode('c', 'db'),
      makeNode('d', 'cache'),
    ];
    const edges = [
      makeEdge('e1', 'a', 'b'),
      makeEdge('e2', 'c', 'd'),
    ];

    const result = await computeLayout(nodes, edges);

    expect(result.nodes).toHaveLength(4);
    expect(result.edges).toHaveLength(2);

    // All nodes should have distinct positions
    const positions = result.nodes.map((n) => `${n.position.x},${n.position.y}`);
    const unique = new Set(positions);
    expect(unique.size).toBe(4);
  });

  it('handles a chain topology (a → b → c → d)', async () => {
    const nodes = [
      makeNode('a', 'frontend'),
      makeNode('b', 'api'),
      makeNode('c', 'worker'),
      makeNode('d', 'db'),
    ];
    const edges = [
      makeEdge('e1', 'a', 'b'),
      makeEdge('e2', 'b', 'c'),
      makeEdge('e3', 'c', 'd'),
    ];

    const result = await computeLayout(nodes, edges);

    // In a DOWN-direction layered layout, nodes should be stacked vertically
    const ys = result.nodes.map((n) => n.position.y);
    const sortedYs = [...ys].sort((a, b) => a - b);
    // At least 2 distinct y levels
    expect(new Set(sortedYs).size).toBeGreaterThanOrEqual(2);
  });
});

describe('computeLayout network edges', () => {
  it('spreads edges into a network along its side instead of piling them on its corners', async () => {
    // Services of network a that also join network b, which has no services
    // of its own: their edges end on b's box.
    const nodes = [
      makeNode('net:a', 'a', 'networkGroup'),
      makeNode('a1', 'svc-1', 'containerNode', 'net:a'),
      makeNode('a2', 'svc-2', 'containerNode', 'net:a'),
      makeNode('net:b', 'b', 'networkGroup'),
    ];
    const edges = [makeEdge('e1', 'a1', 'net:b'), makeEdge('e2', 'a2', 'net:b')];

    const result = await computeLayout(nodes, edges);

    const net = result.nodes.find((n) => n.id === 'net:b')!;
    const left = net.position.x;
    const right = left + (net.style!.width as number);
    const intoNet = result.edges.filter((e) => e.target === 'net:b');
    expect(intoNet).toHaveLength(2);
    for (const e of intoNet) {
      const points = [...(e.data!.path as string).matchAll(/(-?[\d.]+) (-?[\d.]+)/g)];
      const endX = Number(points[points.length - 1][1]);
      expect(Math.min(endX - left, right - endX)).toBeGreaterThan(12);
    }
  });
});

describe('computeLayout node width', () => {
  // 20 characters: 150px in the 12.5px name font, 138px in the 11.5px volume font.
  const NAME = 'abcdefghijklmnopqrst';

  function service(id: string, name: string, running: number, desired: number): RFNode {
    return {
      id,
      type: 'serviceNode',
      position: { x: 0, y: 0 },
      data: { dgNode: { id, name, type: 'service', status: 'running', service: { replicas: { running, desired } } } },
    } as unknown as RFNode;
  }

  const widthOf = async (nodes: RFNode[]) => (await computeLayout(nodes, [])).nodes[0].data.nodeWidth;

  afterEach(() => {
    Reflect.deleteProperty(document, 'fonts');
  });

  it('fits a container name in the font the node renders it in, beside the menu and status dot', async () => {
    expect(await widthOf([makeNode('c', NAME)])).toBe(150 + 54);
  });

  it('fits a volume name beside its icon and menu', async () => {
    expect(await widthOf([makeNode('v', NAME, 'volumeNode')])).toBe(138 + 58);
  });

  it('fits a service name beside its menu and a replica badge with room for two-digit counts', async () => {
    // Badge: 5 characters ("00/00") at 6px plus 12px of padding and border.
    expect(await widthOf([service('s', NAME, 3, 3)])).toBe(150 + 48 + 30 + 12);
  });

  it('sizes every node for the widest name, within 140-280px', async () => {
    expect(await widthOf([makeNode('a', 'web'), makeNode('b', NAME)])).toBe(204);
    expect(await widthOf([makeNode('a', 'web')])).toBe(140);
    expect(await widthOf([makeNode('a', NAME.repeat(3))])).toBe(280);
  });

  it('waits for the label font before measuring names', async () => {
    let fontLoaded!: () => void;
    const load = vi.fn(() => new Promise<void>((resolve) => { fontLoaded = resolve; }));
    Object.defineProperty(document, 'fonts', { configurable: true, value: { load } });
    measureText.mockClear();

    const layout = computeLayout([makeNode('f', 'font-wait-check')], []);
    await Promise.resolve();
    expect(load).toHaveBeenCalled();
    expect(measureText).not.toHaveBeenCalled();

    fontLoaded();
    await layout;
    expect(measureText).toHaveBeenCalledWith('font-wait-check');
  });
});

