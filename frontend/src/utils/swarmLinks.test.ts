import { describe, it, expect } from 'vitest';
import type { Node as RFNode } from '@xyflow/react';
import {
  controlLinkEdges,
  controlLinkId,
  overlayLinkEdges,
  MAX_OVERLAY_LINKS,
  type ControlLinkData,
  type OverlayLinkData,
} from './swarmLinks';
import { toNodeGroupedFlowNodes, roleGroupId } from './nodeTransform';
import type { DGEdge, DGNode, SwarmNodeInfo, TaskInfo } from '../types';

function swarmNode(name: string, role: 'manager' | 'worker', state = 'ready', availability: SwarmNodeInfo['availability'] = 'active'): DGNode {
  return {
    id: `swarmnode:${name}`, type: 'swarmnode', name, status: state,
    swarmNode: { id: `id-${name}`, role, leader: role === 'manager', availability, state },
  };
}

function service(name: string, hosts: string[], networkId?: string): DGNode {
  const tasks: TaskInfo[] = hosts.map((h, i) => ({
    id: `${name}-${i}`, slot: i + 1, nodeHostname: h, state: 'running', desiredState: 'running',
  }));
  return {
    id: `service:${name}`, type: 'service', name, status: 'running', stack: 'wp', networkId,
    service: { mode: 'replicated', replicas: { running: hosts.length, desired: hosts.length }, tasks },
  };
}

const front: DGNode = { id: 'network:wp_front', type: 'network', name: 'wp_front', stack: 'wp' };
const back: DGNode = { id: 'network:wp_back', type: 'network', name: 'wp_back', stack: 'wp' };

describe('controlLinkEdges', () => {
  it('is empty without workers or without managers', () => {
    expect(controlLinkEdges([swarmNode('m1', 'manager')])).toEqual([]);
    expect(controlLinkEdges([swarmNode('w1', 'worker')])).toEqual([]);
  });

  it('draws one spoke per worker from the Managers group, healthy when ready', () => {
    const edges = controlLinkEdges([swarmNode('m1', 'manager'), swarmNode('w1', 'worker'), swarmNode('w2', 'worker')]);
    expect(edges.map((e) => e.id)).toEqual([controlLinkId('swarmnode:w1'), controlLinkId('swarmnode:w2')]);
    expect(edges.every((e) => e.source === roleGroupId('manager'))).toBe(true);
    expect(edges.map((e) => e.target)).toEqual(['swarmnode:w1', 'swarmnode:w2']);
    expect(edges.map((e) => e.data as unknown as ControlLinkData)).toEqual([
      { kind: 'control', healthy: true },
      { kind: 'control', healthy: true },
    ]);
  });

  it('flags a down worker on its own spoke, but not a drained one', () => {
    const edges = controlLinkEdges([
      swarmNode('m1', 'manager'),
      swarmNode('w1', 'worker', 'down'),
      swarmNode('w2', 'worker', 'ready', 'drain'),
    ]);
    expect(edges.map((e) => (e.data as unknown as ControlLinkData).healthy)).toEqual([false, true]);
  });
});

describe('overlayLinkEdges', () => {
  const nodes = [swarmNode('m1', 'manager'), swarmNode('w1', 'worker'), swarmNode('w2', 'worker')];
  const nginx = service('wp_nginx', ['m1', 'w1'], front.id);
  const wordpress = service('wp_wordpress', ['m1', 'w2'], front.id);
  const mariadb = service('wp_mariadb', ['w1'], back.id);
  const lonely = service('wp_lonely', ['w2']);
  // wordpress also reaches the database over the back network.
  const edges: DGEdge[] = [{ id: 'e1', type: 'secondary_network', source: wordpress.id, target: back.id }];
  const dg = [...nodes, nginx, wordpress, mariadb, lonely, front, back];
  const rf = toNodeGroupedFlowNodes(dg);
  const pairs = (list: ReturnType<typeof overlayLinkEdges>) => list.map((e) => `${e.source}->${e.target}`).sort();

  it('links cards of network peers on other nodes, skipping same-node pairs', () => {
    expect(pairs(overlayLinkEdges(dg, edges, rf, nginx.id))).toEqual([
      'nodesvc:m1:wp_nginx->nodesvc:w1:wp_nginx',
      'nodesvc:m1:wp_nginx->nodesvc:w2:wp_wordpress',
      'nodesvc:w1:wp_nginx->nodesvc:m1:wp_wordpress',
      'nodesvc:w1:wp_nginx->nodesvc:w2:wp_wordpress',
    ]);
  });

  it('follows secondary networks and names the shared ones', () => {
    const links = overlayLinkEdges(dg, edges, rf, mariadb.id);
    expect(pairs(links)).toEqual([
      'nodesvc:w1:wp_mariadb->nodesvc:m1:wp_wordpress',
      'nodesvc:w1:wp_mariadb->nodesvc:w2:wp_wordpress',
    ]);
    expect((links[0].data as unknown as OverlayLinkData).networks).toEqual(['wp_back']);
  });

  it('draws nothing for a service without networks', () => {
    expect(overlayLinkEdges(dg, edges, rf, lonely.id)).toEqual([]);
  });

  it('caps the number of links', () => {
    const hosts = Array.from({ length: 12 }, (_, i) => `w${i}`);
    const many = [
      swarmNode('m1', 'manager'),
      ...hosts.map((h) => swarmNode(h, 'worker')),
      service('wp_a', hosts, front.id),
      service('wp_b', hosts, front.id),
      front,
    ];
    const links = overlayLinkEdges(many, [], toNodeGroupedFlowNodes(many), 'service:wp_a');
    expect(links).toHaveLength(MAX_OVERLAY_LINKS);
    expect(new Set(links.map((e) => e.id)).size).toBe(MAX_OVERLAY_LINKS);
  });

  it('ignores nodes that are not service cards', () => {
    const extra: RFNode = { id: 'container:x', type: 'containerNode', position: { x: 0, y: 0 }, data: {} };
    expect(overlayLinkEdges(dg, edges, [...rf, extra], nginx.id)).toHaveLength(4);
  });
});
