import { describe, it, expect } from 'vitest';
import type { DGNode, DGEdge } from '../types';
import { applyDelta, snapshotFingerprint } from './deltaUtils';

const web: DGNode = { id: 'container:web', type: 'container', name: 'web', status: 'running' };
const db: DGNode = { id: 'container:db', type: 'container', name: 'db', status: 'running' };
const edge1: DGEdge = { id: 'e:dep:web:db', type: 'depends_on', source: 'container:web', target: 'container:db' };

describe('applyDelta', () => {
  it('adds new nodes', () => {
    const result = applyDelta(
      { nodes: [web], edges: [] },
      { nodesAdded: [db] },
    );
    expect(result.nodes).toHaveLength(2);
    expect(result.nodes[1].id).toBe('container:db');
  });

  it('removes nodes by id', () => {
    const result = applyDelta(
      { nodes: [web, db], edges: [] },
      { nodesRemoved: ['container:db'] },
    );
    expect(result.nodes).toHaveLength(1);
    expect(result.nodes[0].id).toBe('container:web');
  });

  it('updates existing nodes in place', () => {
    const result = applyDelta(
      { nodes: [web], edges: [] },
      { nodesUpdated: [{ ...web, status: 'exited' }] },
    );
    expect(result.nodes[0].status).toBe('exited');
  });

  it('ignores update for nonexistent node', () => {
    const ghost: DGNode = { id: 'container:ghost', type: 'container', name: 'ghost', status: 'exited' };
    const result = applyDelta(
      { nodes: [web], edges: [] },
      { nodesUpdated: [ghost] },
    );
    expect(result.nodes).toHaveLength(1);
    expect(result.nodes[0].id).toBe('container:web');
  });

  it('adds new edges', () => {
    const result = applyDelta(
      { nodes: [web, db], edges: [] },
      { edgesAdded: [edge1] },
    );
    expect(result.edges).toHaveLength(1);
    expect(result.edges[0].id).toBe('e:dep:web:db');
  });

  it('removes edges by id', () => {
    const result = applyDelta(
      { nodes: [web, db], edges: [edge1] },
      { edgesRemoved: ['e:dep:web:db'] },
    );
    expect(result.edges).toHaveLength(0);
  });

  it('applies mixed delta (add, remove, update)', () => {
    const cache: DGNode = { id: 'container:cache', type: 'container', name: 'cache', status: 'running' };
    const result = applyDelta(
      { nodes: [web, db], edges: [edge1] },
      {
        nodesRemoved: ['container:db'],
        nodesAdded: [cache],
        nodesUpdated: [{ ...web, status: 'exited' }],
        edgesRemoved: ['e:dep:web:db'],
      },
    );
    expect(result.nodes).toHaveLength(2);
    expect(result.nodes.find((n) => n.id === 'container:web')?.status).toBe('exited');
    expect(result.nodes.find((n) => n.id === 'container:cache')).toBeDefined();
    expect(result.nodes.find((n) => n.id === 'container:db')).toBeUndefined();
    expect(result.edges).toHaveLength(0);
  });

  it('handles empty delta as no-op', () => {
    const result = applyDelta({ nodes: [web], edges: [edge1] }, {});
    expect(result.nodes).toHaveLength(1);
    expect(result.edges).toHaveLength(1);
  });

  it('does not mutate the original arrays', () => {
    const nodes = [web, db];
    const edges = [edge1];
    applyDelta({ nodes, edges }, { nodesRemoved: ['container:db'] });
    expect(nodes).toHaveLength(2);
    expect(edges).toHaveLength(1);
  });
});

describe('snapshotFingerprint', () => {
  it('produces identical fingerprints for identical data', () => {
    const fp1 = snapshotFingerprint([web], [edge1]);
    const fp2 = snapshotFingerprint([web], [edge1]);
    expect(fp1).toBe(fp2);
  });

  it('produces different fingerprints for different statuses', () => {
    const fp1 = snapshotFingerprint([web], []);
    const fp2 = snapshotFingerprint([{ ...web, status: 'exited' }], []);
    expect(fp1).not.toBe(fp2);
  });

  it('produces different fingerprints for different images', () => {
    const fp1 = snapshotFingerprint([{ ...web, image: 'nginx:1.0' }], []);
    const fp2 = snapshotFingerprint([{ ...web, image: 'nginx:2.0' }], []);
    expect(fp1).not.toBe(fp2);
  });

  it('produces different fingerprints for different ports', () => {
    const fp1 = snapshotFingerprint([{ ...web, ports: [{ host: 80, container: 80 }] }], []);
    const fp2 = snapshotFingerprint([{ ...web, ports: [{ host: 8080, container: 80 }] }], []);
    expect(fp1).not.toBe(fp2);
  });

  it('produces different fingerprints for different networkIds', () => {
    const fp1 = snapshotFingerprint([{ ...web, networkId: 'network:a' }], []);
    const fp2 = snapshotFingerprint([{ ...web, networkId: 'network:b' }], []);
    expect(fp1).not.toBe(fp2);
  });

  it('handles empty arrays', () => {
    const fp = snapshotFingerprint([], []);
    expect(fp).toBe('|');
  });
});
