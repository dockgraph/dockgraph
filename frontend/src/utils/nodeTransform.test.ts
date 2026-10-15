import { describe, it, expect } from 'vitest';
import {
  toNodeGroupedFlowNodes,
  nodeGroupedTopologyKey,
  placeTasks,
  isActiveTask,
  nodeUsage,
  UNASSIGNED_NODE_GROUP_ID,
  CONTROL_SUMMARY_ID,
  roleGroupId,
  isSwarmNodeInactive,
} from './nodeTransform';
import { filterGraphByStack } from './stack';
import type { DGNode, NodeServiceCardData, SwarmNodeGroupData, RoleGroupData, ControlSummaryData } from '../types';

const mgr: DGNode = {
  id: 'swarmnode:mgr', type: 'swarmnode', name: 'mgr', status: 'ready',
  swarmNode: { id: 'n1', role: 'manager', leader: true, availability: 'active', state: 'ready', nanoCpus: 4e9, memoryBytes: 8e9 },
};
const wrk: DGNode = {
  id: 'swarmnode:wrk', type: 'swarmnode', name: 'wrk', status: 'ready',
  swarmNode: { id: 'n2', role: 'worker', leader: false, availability: 'drain', state: 'ready' },
};
const web: DGNode = {
  id: 'service:shop_web', type: 'service', name: 'shop_web', status: 'running', stack: 'shop',
  service: {
    mode: 'replicated',
    replicas: { running: 2, desired: 3 },
    tasks: [
      { id: 't2', slot: 2, nodeId: 'n2', nodeHostname: 'wrk', state: 'running', desiredState: 'running' },
      { id: 't1', slot: 1, nodeId: 'n1', nodeHostname: 'mgr', state: 'running', desiredState: 'running' },
      { id: 't3', slot: 3, state: 'pending', desiredState: 'running' },
      { id: 'old', slot: 1, nodeId: 'n1', nodeHostname: 'mgr', state: 'shutdown', desiredState: 'shutdown' },
    ],
  },
};
const api: DGNode = {
  id: 'service:blog_api', type: 'service', name: 'blog_api', status: 'running', stack: 'blog',
  service: {
    mode: 'global',
    replicas: { running: 1, desired: 1 },
    // Matched by node id only (no hostname).
    tasks: [{ id: 'g1', nodeId: 'n1', state: 'running', desiredState: 'running' }],
  },
};
const lone: DGNode = { id: 'container:lone', type: 'container', name: 'lone', status: 'running' };
const ghost: DGNode = { id: 'container:ghost', type: 'container', name: 'ghost', status: 'not_running' };
const net: DGNode = { id: 'network:shop_net', type: 'network', name: 'shop_net', stack: 'shop' };

const all = [web, api, lone, ghost, net, wrk, mgr];

describe('toNodeGroupedFlowNodes', () => {
  it('nests boxes in Managers / Workers role groups, leader first, and adds a free Unassigned box', () => {
    const rf = toNodeGroupedFlowNodes(all, 'n1');
    const roles = rf.filter((n) => n.type === 'roleGroup');
    expect(roles.map((g) => g.id)).toEqual([roleGroupId('manager'), roleGroupId('worker')]);
    expect(roles.every((g) => g.draggable === true)).toBe(true);
    expect(roles[0].data as unknown as RoleGroupData).toEqual({ role: 'manager', nodeCount: 1, taskCount: 3 });
    expect(roles[1].data as unknown as RoleGroupData).toEqual({ role: 'worker', nodeCount: 1, taskCount: 1 });

    const boxes = rf.filter((n) => n.type === 'nodeGroup');
    expect(boxes.map((g) => [g.id, g.parentId])).toEqual([
      ['swarmnode:mgr', 'rolegroup:manager'],
      ['swarmnode:wrk', 'rolegroup:worker'],
      [UNASSIGNED_NODE_GROUP_ID, undefined],
    ]);
    expect(boxes.every((b) => b.draggable === false)).toBe(true);
    expect((boxes[0].data as unknown as SwarmNodeGroupData).role).toBe('manager');
    expect((boxes[2].data as unknown as SwarmNodeGroupData).unassigned).toBe(true);
  });

  it('lists parents before their children', () => {
    const rf = toNodeGroupedFlowNodes(all, 'n1');
    const index = new Map(rf.map((n, i) => [n.id, i]));
    for (const n of rf) {
      if (n.parentId) expect(index.get(n.parentId)!).toBeLessThan(index.get(n.id)!);
    }
  });

  it('omits a role group without nodes', () => {
    const rf = toNodeGroupedFlowNodes([web, mgr], null);
    expect(rf.filter((n) => n.type === 'roleGroup').map((n) => n.id)).toEqual(['rolegroup:manager']);
  });

  it('adds a control summary badge when both a manager and a worker exist', () => {
    const rf = toNodeGroupedFlowNodes(all, 'n1');
    const summary = rf.find((n) => n.id === CONTROL_SUMMARY_ID)!;
    expect(summary.type).toBe('controlSummary');
    expect(summary.draggable).toBe(false);
    expect(summary.selectable).toBe(false);
    // wrk is drained but still "ready" — counts as up, like the old aggregate link did.
    expect(summary.data as unknown as ControlSummaryData).toEqual({ ready: 1, total: 1, healthy: true });
  });

  it('flags a down worker in the control summary', () => {
    const down = { ...wrk, swarmNode: { ...wrk.swarmNode!, state: 'down' } };
    const rf = toNodeGroupedFlowNodes([mgr, down], null);
    const summary = rf.find((n) => n.id === CONTROL_SUMMARY_ID)!;
    expect(summary.data as unknown as ControlSummaryData).toEqual({ ready: 0, total: 1, healthy: false });
  });

  it('omits the control summary without both a manager and a worker', () => {
    expect(toNodeGroupedFlowNodes([web, mgr], null).some((n) => n.id === CONTROL_SUMMARY_ID)).toBe(false);
    expect(toNodeGroupedFlowNodes([wrk], null).some((n) => n.id === CONTROL_SUMMARY_ID)).toBe(false);
  });

  it('draws one card per service on each node, with stable ids, sorted by stack then service', () => {
    const rf = toNodeGroupedFlowNodes(all, 'n1');
    const inMgr = rf.filter((n) => n.parentId === 'swarmnode:mgr');
    // blog < shop; standalone containers after the cards.
    expect(inMgr.map((n) => n.id)).toEqual(['nodesvc:mgr:blog_api', 'nodesvc:mgr:shop_web', 'container:lone']);
    expect(rf.filter((n) => n.parentId === 'swarmnode:wrk').map((n) => n.id)).toEqual(['nodesvc:wrk:shop_web']);
    expect(rf.filter((n) => n.parentId === UNASSIGNED_NODE_GROUP_ID).map((n) => n.id)).toEqual(['nodesvc:*:shop_web']);

    const card = rf.find((n) => n.id === 'nodesvc:mgr:shop_web')!;
    expect(card.type).toBe('nodeServiceCard');
    const d = card.data as unknown as NodeServiceCardData;
    expect(d.serviceId).toBe('service:shop_web');
    expect(d.serviceName).toBe('shop_web');
    expect(d.stack).toBe('shop');
    // Only running-desired tasks: the shut-down "old" task is gone.
    expect(d.tasks.map((t) => t.id)).toEqual(['t1']);
    expect((rf.find((n) => n.id === 'swarmnode:mgr')!.data as unknown as SwarmNodeGroupData).taskCount).toBe(3);
  });

  it('groups several tasks of a service on one node into one card, by slot', () => {
    const multi: DGNode = {
      ...web,
      service: {
        ...web.service!,
        tasks: [
          { id: 'b', slot: 3, nodeHostname: 'wrk', state: 'running', desiredState: 'running' },
          { id: 'a', slot: 1, nodeHostname: 'wrk', state: 'starting', desiredState: 'running' },
        ],
      },
    };
    const rf = toNodeGroupedFlowNodes([multi, mgr, wrk], null);
    const cards = rf.filter((n) => n.type === 'nodeServiceCard');
    expect(cards.map((c) => c.id)).toEqual(['nodesvc:wrk:shop_web']);
    expect((cards[0].data as unknown as NodeServiceCardData).tasks.map((t) => t.slot)).toEqual([1, 3]);
  });

  it('marks cards on drained or down nodes as inactive', () => {
    const rf = toNodeGroupedFlowNodes(all, 'n1');
    const on = (id: string) => (rf.find((n) => n.id === id)!.data as unknown as NodeServiceCardData).nodeInactive;
    expect(on('nodesvc:wrk:shop_web')).toBe(true); // wrk is drained
    expect(on('nodesvc:mgr:shop_web')).toBeUndefined();
    expect(isSwarmNodeInactive({ ...mgr, swarmNode: { ...mgr.swarmNode!, state: 'down' } })).toBe(true);
    expect(isSwarmNodeInactive(mgr)).toBe(false);
  });

  it('hides networks, volumes and not-running containers', () => {
    const rf = toNodeGroupedFlowNodes(all, 'n1');
    expect(rf.some((n) => n.id === 'network:shop_net')).toBe(false);
    expect(rf.some((n) => n.id === 'container:ghost')).toBe(false);
  });

  it('leaves standalone containers free-standing while the local node is unknown', () => {
    const rf = toNodeGroupedFlowNodes(all, null);
    const lc = rf.find((n) => n.id === 'container:lone')!;
    expect(lc.parentId).toBeUndefined();
    expect((rf.find((n) => n.id === 'swarmnode:mgr')!.data as unknown as SwarmNodeGroupData).taskCount).toBe(2);
  });

  it('keeps empty node boxes when scoped to a stack', () => {
    const { nodes } = filterGraphByStack(all, [], 'blog');
    const rf = toNodeGroupedFlowNodes(nodes, 'n1');
    expect(rf.filter((n) => n.type === 'nodeGroup').map((n) => n.id)).toEqual(['swarmnode:mgr', 'swarmnode:wrk']);
    expect(rf.filter((n) => n.type === 'nodeServiceCard').map((n) => n.id)).toEqual(['nodesvc:mgr:blog_api']);
    expect(rf.some((n) => n.id === 'container:lone')).toBe(false);
  });
});

describe('nodeGroupedTopologyKey', () => {
  const withTasks = (tasks: NonNullable<DGNode['service']>['tasks']): DGNode => ({ ...web, service: { ...web.service!, tasks } });

  it('changes when a task moves to another node', () => {
    const before = nodeGroupedTopologyKey(all, 'n1');
    const moved = withTasks(web.service!.tasks!.map((t) => (t.id === 't2' ? { ...t, nodeId: 'n1', nodeHostname: 'mgr' } : t)));
    expect(nodeGroupedTopologyKey([moved, api, lone, wrk, mgr], 'n1')).not.toBe(before);
  });

  it('ignores task state changes and restarts that keep the per-node count', () => {
    const base = [web, api, lone, wrk, mgr];
    const before = nodeGroupedTopologyKey(base, 'n1');
    const restarted = withTasks(web.service!.tasks!.map((t) =>
      t.id === 't1' ? { ...t, id: 't1-new', state: 'starting' } : t));
    expect(nodeGroupedTopologyKey([restarted, api, lone, wrk, mgr], 'n1')).toBe(before);
  });

  it('changes when a card gains a task row', () => {
    const before = nodeGroupedTopologyKey(all, 'n1');
    const more = withTasks([...web.service!.tasks!, { id: 't4', slot: 4, nodeHostname: 'wrk', state: 'running', desiredState: 'running' }]);
    expect(nodeGroupedTopologyKey([more, api, lone, ghost, net, wrk, mgr], 'n1')).not.toBe(before);
  });
});

describe('placeTasks', () => {
  it('sends tasks on an unknown node to Unassigned', () => {
    const svc: DGNode = { ...web, service: { ...web.service!, tasks: [{ id: 'x', slot: 1, nodeId: 'gone', desiredState: 'running' }] } };
    expect(placeTasks([svc, mgr]).get(UNASSIGNED_NODE_GROUP_ID)?.map((p) => p.task.id)).toEqual(['x']);
  });
});

describe('nodeUsage', () => {
  it('relates stats to capacity', () => {
    expect(nodeUsage(mgr.swarmNode, { cpuPercent: 200, memUsage: 2e9 })).toEqual({ cores: 4, cpuPercent: 50, memPercent: 25 });
    expect(nodeUsage(wrk.swarmNode, undefined)).toEqual({ cores: undefined });
  });
});

describe('placeTasks active-task filter', () => {
  const job: DGNode = {
    id: 'service:ops_migrate', type: 'service', name: 'ops_migrate', status: 'running', stack: 'ops',
    service: {
      mode: 'replicated-job',
      replicas: { running: 0, desired: 1 },
      tasks: [
        { id: 'run', slot: 1, nodeHostname: 'mgr', state: 'running', desiredState: 'complete' },
        { id: 'done', slot: 2, nodeHostname: 'mgr', state: 'complete', desiredState: 'complete' },
        { id: 'gone', slot: 3, nodeHostname: 'mgr', state: 'complete', desiredState: 'remove' },
        { id: 'failed', slot: 4, nodeHostname: 'mgr', state: 'failed', desiredState: 'shutdown' },
      ],
    },
  };

  it('keeps running job tasks and drops completed, shutdown and removed ones', () => {
    const ids = (placeTasks([mgr, job]).get('swarmnode:mgr') ?? []).map((p) => p.task.id);
    expect(ids).toEqual(['run']);
  });

  it('matches the backend check for each desired state', () => {
    const t = (desiredState: string) => ({ id: 'x', desiredState });
    expect(['running', 'ready', 'complete', ''].map((s) => isActiveTask(t(s)))).toEqual([true, true, true, true]);
    expect(['shutdown', 'remove'].map((s) => isActiveTask(t(s)))).toEqual([false, false]);
  });

  it('treats a finished job task as history, whatever its desired state', () => {
    const t = (state: string, desiredState: string) => ({ id: 'x', state, desiredState });
    expect(isActiveTask(t('running', 'complete'))).toBe(true);
    expect(isActiveTask(t('pending', 'complete'))).toBe(true);
    expect(isActiveTask(t('complete', 'complete'))).toBe(false);
    expect(isActiveTask(t('complete', 'running'))).toBe(false);
  });
});
