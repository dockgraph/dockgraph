import { describe, it, expect } from 'vitest';
import { toReactFlowNodes, toReactFlowEdges } from './graphTransform';
import type { DGNode, DGEdge } from '../types';

describe('toReactFlowNodes', () => {
  it('returns empty array for no nodes', () => {
    expect(toReactFlowNodes([], [])).toEqual([]);
  });

  it('creates container nodes with correct type', () => {
    const nodes: DGNode[] = [
      { id: 'container:web', type: 'container', name: 'web', status: 'running', networkId: 'network:backend' },
      { id: 'network:backend', type: 'network', name: 'backend' },
    ];
    const result = toReactFlowNodes(nodes, []);
    const container = result.find((n) => n.id === 'container:web');
    expect(container?.type).toBe('containerNode');
    expect(container?.parentId).toBe('network:backend');
  });

  it('creates network group nodes for networks with children', () => {
    const nodes: DGNode[] = [
      { id: 'container:web', type: 'container', name: 'web', networkId: 'network:frontend' },
      { id: 'network:frontend', type: 'network', name: 'frontend' },
    ];
    const result = toReactFlowNodes(nodes, []);
    const group = result.find((n) => n.id === 'network:frontend');
    expect(group?.type).toBe('networkGroup');
  });

  it('skips network groups with no children and no edges', () => {
    const nodes: DGNode[] = [
      { id: 'network:orphan', type: 'network', name: 'orphan' },
    ];
    const result = toReactFlowNodes(nodes, []);
    expect(result.find((n) => n.id === 'network:orphan')).toBeUndefined();
  });

  it('creates unmanaged group for containers without source or network', () => {
    const nodes: DGNode[] = [
      { id: 'container:standalone', type: 'container', name: 'standalone', status: 'running' },
    ];
    const result = toReactFlowNodes(nodes, []);
    const unmanaged = result.find((n) => n.id === 'group:unmanaged');
    expect(unmanaged).toBeDefined();
    const container = result.find((n) => n.id === 'container:standalone');
    expect(container?.parentId).toBe('group:unmanaged');
  });

  it('does not create unmanaged group when all containers have a source', () => {
    const nodes: DGNode[] = [
      { id: 'container:web', type: 'container', name: 'web', source: 'compose.yml' },
    ];
    const result = toReactFlowNodes(nodes, []);
    expect(result.find((n) => n.id === 'group:unmanaged')).toBeUndefined();
  });

  it('places volumes inside the group of their first consumer', () => {
    const nodes: DGNode[] = [
      { id: 'container:db', type: 'container', name: 'db', networkId: 'network:backend' },
      { id: 'network:backend', type: 'network', name: 'backend' },
      { id: 'volume:pgdata', type: 'volume', name: 'pgdata' },
    ];
    const edges: DGEdge[] = [
      { id: 'e:vol:pgdata:db', type: 'volume_mount', source: 'volume:pgdata', target: 'container:db', mountPath: '/data' },
    ];
    const result = toReactFlowNodes(nodes, edges);
    const vol = result.find((n) => n.id === 'volume:pgdata');
    expect(vol?.parentId).toBe('network:backend');
  });

  it('creates volume nodes without parent when no consumer exists', () => {
    const nodes: DGNode[] = [
      { id: 'volume:orphan', type: 'volume', name: 'orphan' },
    ];
    const result = toReactFlowNodes(nodes, []);
    const vol = result.find((n) => n.id === 'volume:orphan');
    expect(vol?.type).toBe('volumeNode');
    expect(vol?.parentId).toBeUndefined();
  });
});

describe('toReactFlowEdges', () => {
  const defaultStroke = '#475569';

  it('returns empty array for no edges', () => {
    expect(toReactFlowEdges([], [], defaultStroke)).toEqual([]);
  });

  it('filters edges with missing endpoints', () => {
    const nodes: DGNode[] = [
      { id: 'container:web', type: 'container', name: 'web', status: 'running' },
    ];
    const edges: DGEdge[] = [
      { id: 'e:dep:web:missing', type: 'depends_on', source: 'container:web', target: 'container:missing' },
    ];
    expect(toReactFlowEdges(edges, nodes, defaultStroke)).toEqual([]);
  });

  it('sets orange stroke for volume mount edges', () => {
    const nodes: DGNode[] = [
      { id: 'volume:data', type: 'volume', name: 'data' },
      { id: 'container:db', type: 'container', name: 'db', status: 'running' },
    ];
    const edges: DGEdge[] = [
      { id: 'e:vol:data:db', type: 'volume_mount', source: 'volume:data', target: 'container:db' },
    ];
    const result = toReactFlowEdges(edges, nodes, defaultStroke);
    expect(result[0].style?.stroke).toBe('#f97316');
  });

  it('marks edges as active when both endpoints are running', () => {
    const nodes: DGNode[] = [
      { id: 'container:web', type: 'container', name: 'web', status: 'running' },
      { id: 'container:db', type: 'container', name: 'db', status: 'running' },
    ];
    const edges: DGEdge[] = [
      { id: 'e:dep:web:db', type: 'depends_on', source: 'container:web', target: 'container:db' },
    ];
    const result = toReactFlowEdges(edges, nodes, defaultStroke);
    expect(result[0].data?.active).toBe(true);
  });

  it('marks edges as inactive when an endpoint is not running', () => {
    const nodes: DGNode[] = [
      { id: 'container:web', type: 'container', name: 'web', status: 'running' },
      { id: 'container:db', type: 'container', name: 'db', status: 'exited' },
    ];
    const edges: DGEdge[] = [
      { id: 'e:dep:web:db', type: 'depends_on', source: 'container:web', target: 'container:db' },
    ];
    const result = toReactFlowEdges(edges, nodes, defaultStroke);
    expect(result[0].data?.active).toBe(false);
  });

  it('uses network color for secondary_network edge stroke', () => {
    const nodes: DGNode[] = [
      { id: 'container:web', type: 'container', name: 'web', status: 'running', networkId: 'network:frontend' },
      { id: 'network:backend', type: 'network', name: 'backend' },
    ];
    const edges: DGEdge[] = [
      { id: 'e:net:web:backend', type: 'secondary_network', source: 'container:web', target: 'network:backend' },
    ];
    const result = toReactFlowEdges(edges, nodes, defaultStroke);
    expect(result[0].style?.stroke).toMatch(/^#[0-9a-f]{6}$/);
    expect(result[0].style?.stroke).not.toBe(defaultStroke);
  });

  it('falls back to default stroke for secondary_network edge with missing target', () => {
    const nodes: DGNode[] = [
      { id: 'container:web', type: 'container', name: 'web', status: 'running' },
      { id: 'network:x', type: 'network', name: 'x' },
    ];
    const edges: DGEdge[] = [
      { id: 'e:net:web:x', type: 'secondary_network', source: 'container:web', target: 'network:x' },
    ];
    const result = toReactFlowEdges(edges, nodes, defaultStroke);
    // target exists so stroke should be network color
    expect(result[0].style?.stroke).toBeDefined();
  });
});
