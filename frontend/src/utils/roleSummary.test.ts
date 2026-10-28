import { describe, it, expect } from 'vitest';
import { summarizeRole, quorumHealth } from './roleSummary';
import { boxTaskCounts } from './nodeTransform';
import { nodeStatsKey } from './stack';
import type { DGNode, SwarmNodeInfo } from '../types';
import type { ContainerStatsData, SwarmManager } from '../types/stats';

function swarmNode(name: string, info: Partial<SwarmNodeInfo>): DGNode {
  const swarmNode: SwarmNodeInfo = { id: `id-${name}`, role: 'manager', availability: 'active', state: 'ready', ...info };
  return { id: `swarmnode:${name}`, type: 'swarmnode', name, status: swarmNode.state, swarmNode };
}

function stats(cpuPercent: number, memUsage: number): ContainerStatsData {
  return {
    cpuPercent, memUsage, cpuThrottled: 0, memLimit: 0, netRx: 0, netTx: 0,
    netRxErrors: 0, netTxErrors: 0, blockRead: 0, blockWrite: 0, pids: 0,
  };
}

const m1 = swarmNode('m1', { leader: true, engineVersion: '28.5.1', nanoCpus: 4e9, memoryBytes: 8e9 });
const m2 = swarmNode('m2', { availability: 'drain', engineVersion: '28.5.1', nanoCpus: 2e9, memoryBytes: 4e9 });
const m3 = swarmNode('m3', { state: 'down', engineVersion: '27.3.1', nanoCpus: 2e9, memoryBytes: 4e9 });
const w1 = swarmNode('w1', { role: 'worker', engineVersion: '28.5.1', nanoCpus: 2e9, memoryBytes: 4e9 });

const web: DGNode = {
  id: 'service:shop_web', type: 'service', name: 'shop_web', status: 'running',
  service: {
    mode: 'replicated',
    replicas: { running: 4, desired: 4 },
    tasks: [
      { id: 't1', slot: 1, nodeHostname: 'm1', state: 'running', desiredState: 'running' },
      { id: 't2', slot: 2, nodeHostname: 'm1', state: 'running', desiredState: 'running' },
      { id: 't3', slot: 3, nodeHostname: 'm2', state: 'running', desiredState: 'running' },
      { id: 't4', slot: 4, nodeHostname: 'w1', state: 'running', desiredState: 'running' },
    ],
  },
};
const lone: DGNode = { id: 'container:lone', type: 'container', name: 'lone', status: 'running' };

// Listed out of display order on purpose.
const graph = [w1, m3, web, m2, lone, m1];
const statsMap = new Map([
  [nodeStatsKey('m1'), stats(200, 2e9)], // 2 of 4 cores
  [nodeStatsKey('m2'), stats(50, 1e9)], // half a core of 2
]);

describe('summarizeRole', () => {
  const counts = boxTaskCounts(graph, 'id-m1');
  const managers = summarizeRole(graph, 'manager', statsMap, counts);

  it("lists the role's nodes in display order with the tasks their boxes show", () => {
    expect(managers.members.map((m) => [m.node.name, m.tasks])).toEqual([['m1', 3], ['m2', 1], ['m3', 0]]);
    expect(managers.tasks).toBe(4);
  });

  it('tallies states, availability and engine versions, most common first', () => {
    expect(managers.states).toEqual([{ value: 'ready', count: 2 }, { value: 'down', count: 1 }]);
    expect(managers.availability).toEqual([{ value: 'active', count: 2 }, { value: 'drain', count: 1 }]);
    expect(managers.engines).toEqual([{ value: '28.5.1', count: 2 }, { value: '27.3.1', count: 1 }]);
  });

  it('sums the capacity of every node, and usage over the nodes that report stats', () => {
    expect(managers.cores).toBe(8);
    expect(managers.memoryBytes).toBe(16e9);
    // m3 runs no agent: usage is relative to m1 and m2 only.
    expect(managers.usage?.reporting).toBe(2);
    expect(managers.usage?.cores).toBe(6);
    expect(managers.usage?.cpuPercent).toBeCloseTo((250 / 6), 5);
    expect(managers.usage?.memPercent).toBeCloseTo(25, 5);
    expect(managers.usage?.memUsed).toBe(3e9);
    expect(managers.usage?.memCapacity).toBe(12e9);
  });

  it('has no usage when no node reports stats', () => {
    const workers = summarizeRole(graph, 'worker', statsMap, counts);
    expect(workers.members.map((m) => m.node.name)).toEqual(['w1']);
    expect(workers.usage).toBeUndefined();
  });
});

describe('quorumHealth', () => {
  const manager = (name: string, reachability: string, leader = false): SwarmManager => ({ id: `id-${name}`, name, reachability, leader });
  const names = (managers: SwarmManager[]) => managers.map((m) => m.name);

  it('keeps a spare manager when all three are reachable', () => {
    expect(quorumHealth([manager('a', 'reachable', true), manager('b', 'reachable'), manager('c', 'reachable')])).toEqual({
      managers: 3, reachable: 3, quorum: 2, spare: 1, leader: 'a', unreachable: [],
    });
  });

  it('has no spare left with one of three unreachable', () => {
    const h = quorumHealth([manager('a', 'reachable', true), manager('b', 'unreachable'), manager('c', 'reachable')]);
    expect([h.reachable, h.spare, names(h.unreachable)]).toEqual([2, 0, ['b']]);
  });

  it('is short of a majority with two of three unreachable', () => {
    const h = quorumHealth([manager('a', 'reachable'), manager('b', 'unreachable'), manager('c', 'unknown')]);
    expect([h.reachable, h.spare, h.leader, names(h.unreachable)]).toEqual([1, -1, undefined, ['b', 'c']]);
  });

  it('needs three of four: an even count tolerates no more failures than one fewer', () => {
    const four = ['a', 'b', 'c', 'd'].map((n) => manager(n, 'reachable'));
    expect([quorumHealth(four).quorum, quorumHealth(four).spare]).toEqual([3, 1]);
    expect(quorumHealth(four.slice(0, 3)).spare).toBe(1);
  });

  it('has no spare with a single manager', () => {
    expect(quorumHealth([manager('a', 'reachable', true)])).toMatchObject({ quorum: 1, spare: 0 });
  });
});
