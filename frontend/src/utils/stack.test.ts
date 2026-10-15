import { describe, it, expect } from 'vitest';
import {
  showsSwarmWorkloads,
  isServiceLive,
  projectOf,
  listStacks,
  filterGraphByStack,
  resolveNodeRef,
  stripStackPrefix,
  taskContainerName,
  STANDALONE_STACK,
} from './stack';
import type { DGNode, DGEdge } from '../types';

const svc = (name: string, stack: string, status = 'running', extra: Partial<DGNode> = {}): DGNode => ({
  id: `service:${name}`, type: 'service', name, stack, status, ...extra,
});

describe('projectOf', () => {
  it('prefers the backend-resolved stack', () => {
    expect(projectOf({ id: 'x', type: 'container', name: 'x', stack: 'shop', labels: { 'com.docker.compose.project': 'other' } })).toBe('shop');
  });

  it('falls back to the compose project label, then the stack namespace label', () => {
    expect(projectOf({ id: 'x', type: 'container', name: 'x', labels: { 'com.docker.compose.project': 'app' } })).toBe('app');
    expect(projectOf({ id: 'x', type: 'container', name: 'x', labels: { 'com.docker.stack.namespace': 'blog' } })).toBe('blog');
  });

  it('returns undefined when the node has no project', () => {
    expect(projectOf({ id: 'x', type: 'container', name: 'x' })).toBeUndefined();
    expect(projectOf({ id: 'x', type: 'container', name: 'x', stack: '' })).toBeUndefined();
  });
});

describe('listStacks', () => {
  it('counts running/total workloads per stack, sorted, standalone last', () => {
    const nodes: DGNode[] = [
      { id: 'container:lone', type: 'container', name: 'lone', status: 'exited' },
      svc('shop_web', 'shop'),
      svc('shop_db', 'shop', 'degraded'),
      { id: 'container:blog-app-1', type: 'container', name: 'blog-app-1', status: 'running', labels: { 'com.docker.compose.project': 'blog' } },
      { id: 'network:shop_net', type: 'network', name: 'shop_net', stack: 'shop' },
    ];
    expect(listStacks(nodes)).toEqual([
      { name: 'blog', running: 1, total: 1 },
      { name: 'shop', running: 1, total: 2 },
      { name: STANDALONE_STACK, running: 0, total: 1 },
    ]);
  });

  it('returns an empty list for a graph without workloads', () => {
    expect(listStacks([{ id: 'network:n', type: 'network', name: 'n' }])).toEqual([]);
  });
});

describe('filterGraphByStack', () => {
  const nodes: DGNode[] = [
    svc('shop_web', 'shop', 'running', { networkId: 'network:shop_front' }),
    svc('shop_db', 'shop', 'running', { networkId: 'network:shop_back' }),
    svc('blog_app', 'blog', 'running', { networkId: 'network:blog_net' }),
    { id: 'container:lone', type: 'container', name: 'lone', status: 'running', networkId: 'network:bridge' },
    { id: 'network:shop_front', type: 'network', name: 'shop_front', stack: 'shop' },
    { id: 'network:shop_back', type: 'network', name: 'shop_back', stack: 'shop' },
    { id: 'network:shop_unused', type: 'network', name: 'shop_unused', stack: 'shop' },
    { id: 'network:blog_net', type: 'network', name: 'blog_net', stack: 'blog' },
    { id: 'network:proxy', type: 'network', name: 'proxy' },
    { id: 'network:bridge', type: 'network', name: 'bridge' },
    { id: 'volume:shop_data', type: 'volume', name: 'shop_data', stack: 'shop' },
    { id: 'volume:shared', type: 'volume', name: 'shared' },
  ];
  const edges: DGEdge[] = [
    { id: 'e:net:shop_web:proxy', type: 'secondary_network', source: 'service:shop_web', target: 'network:proxy' },
    { id: 'e:net:blog_app:proxy', type: 'secondary_network', source: 'service:blog_app', target: 'network:proxy' },
    { id: 'e:vol:shared:shop_db', type: 'volume_mount', source: 'volume:shared', target: 'service:shop_db' },
    { id: 'e:vol:shared:blog_app', type: 'volume_mount', source: 'volume:shared', target: 'service:blog_app' },
    { id: 'e:dep:shop_web:shop_db', type: 'depends_on', source: 'service:shop_web', target: 'service:shop_db' },
  ];

  it('returns the input unchanged when no stack is selected', () => {
    const out = filterGraphByStack(nodes, edges, null);
    expect(out.nodes).toBe(nodes);
    expect(out.edges).toBe(edges);
  });

  it('keeps the stack, its own resources and the external networks/volumes it references', () => {
    const out = filterGraphByStack(nodes, edges, 'shop');
    expect(out.nodes.map((n) => n.id).sort()).toEqual([
      'network:proxy',
      'network:shop_back',
      'network:shop_front',
      'network:shop_unused',
      'service:shop_db',
      'service:shop_web',
      'volume:shared',
      'volume:shop_data',
    ]);
  });

  it('keeps only edges whose ends both survive', () => {
    const out = filterGraphByStack(nodes, edges, 'shop');
    expect(out.edges.map((e) => e.id).sort()).toEqual([
      'e:dep:shop_web:shop_db',
      'e:net:shop_web:proxy',
      'e:vol:shared:shop_db',
    ]);
  });

  it('standalone scope keeps project-less workloads and only the resources they use', () => {
    const out = filterGraphByStack(nodes, edges, STANDALONE_STACK);
    expect(out.nodes.map((n) => n.id).sort()).toEqual(['container:lone', 'network:bridge']);
    expect(out.edges).toEqual([]);
  });

  it('returns an empty graph for an unknown stack', () => {
    const out = filterGraphByStack(nodes, edges, 'nope');
    expect(out.nodes).toEqual([]);
    expect(out.edges).toEqual([]);
  });
});

describe('resolveNodeRef', () => {
  const nodes: DGNode[] = [
    svc('shop_web', 'shop'),
    svc('blog_web', 'blog'),
    svc('shop_db', 'shop'),
    { id: 'container:app-api-1', type: 'container', name: 'app-api-1', stack: 'app' },
    { id: 'container:app-cache', type: 'container', name: 'app-cache', stack: 'app' },
    { id: 'network:shop_net', type: 'network', name: 'shop_net', stack: 'shop' },
  ];

  it('returns exact ids unchanged', () => {
    expect(resolveNodeRef(nodes, 'service:shop_web')).toBe('service:shop_web');
  });

  it('resolves a short name to the {stack}_{svc} service of the current stack', () => {
    expect(resolveNodeRef(nodes, 'container:web', 'blog')).toBe('service:blog_web');
    expect(resolveNodeRef(nodes, 'service:web', 'shop')).toBe('service:shop_web');
  });

  it('resolves a container reference to a same-named service', () => {
    expect(resolveNodeRef(nodes, 'container:shop_db')).toBe('service:shop_db');
  });

  it('maps a task container name to its service', () => {
    expect(resolveNodeRef(nodes, 'container:shop_web.2.x7k2p9q4m1n8b5v3c6z0a1s2d')).toBe('service:shop_web');
  });

  it('only strips a real task suffix (25-char task id)', () => {
    expect(resolveNodeRef(nodes, 'container:shop_web.backup.abcdefghijklmnopqrstu')).toBe('container:shop_web.backup.abcdefghijklmnopqrstu');
  });

  it('still suffix-matches compose names', () => {
    expect(resolveNodeRef(nodes, 'container:cache')).toBe('container:app-cache');
    expect(resolveNodeRef(nodes, 'container:api', 'app')).toBe('container:app-api-1');
    expect(resolveNodeRef(nodes, 'network:net')).toBe('network:shop_net');
  });

  it('returns the target unchanged when nothing matches', () => {
    expect(resolveNodeRef(nodes, 'container:missing')).toBe('container:missing');
    expect(resolveNodeRef(nodes, 'nocolon')).toBe('nocolon');
  });
});

describe('stripStackPrefix', () => {
  it('drops the {stack}_ prefix only when it matches', () => {
    expect(stripStackPrefix('shop_backend', 'shop')).toBe('backend');
    expect(stripStackPrefix('backend', 'shop')).toBe('backend');
    expect(stripStackPrefix('shop_', 'shop')).toBe('shop_');
    expect(stripStackPrefix('shop_backend', undefined)).toBe('shop_backend');
  });
});

describe('taskContainerName', () => {
  it('uses the slot for replicated tasks and the node id for global ones', () => {
    expect(taskContainerName('shop_web', { id: 't1', slot: 2 })).toBe('shop_web.2.t1');
    expect(taskContainerName('shop_agent', { id: 't2', nodeId: 'n1' })).toBe('shop_agent.n1.t2');
  });
});

import { consumerStats, isNodeStatsKey, nodeStatsKey, seriesLabel, serviceOfTask, withoutCoveredTasks, withoutNodeStats } from './stack';
import type { ContainerStatsData } from '../types/stats';

describe('swarm node helpers', () => {
  it('recognises node:{hostname} stats keys', () => {
    expect(nodeStatsKey('mgr')).toBe('node:mgr');
    expect(isNodeStatsKey('node:mgr')).toBe(true);
    expect(isNodeStatsKey('shop_web')).toBe(false);
    expect(seriesLabel('node:mgr')).toBe('mgr');
    expect(seriesLabel('shop_web')).toBe('shop_web');
  });

  it('drops node aggregates from a stats map', () => {
    const s = {} as ContainerStatsData;
    const m = new Map([['web', s], ['node:mgr', s]]);
    expect([...withoutNodeStats(m).keys()]).toEqual(['web']);
    const pure = new Map([['web', s]]);
    expect(withoutNodeStats(pure)).toBe(pure);
  });

  it('keeps swarm nodes when filtering by stack and does not count them as workloads', () => {
    const nodes: DGNode[] = [
      { id: 'swarmnode:mgr', type: 'swarmnode', name: 'mgr', status: 'ready' },
      { id: 'service:shop_web', type: 'service', name: 'shop_web', stack: 'shop', status: 'running' },
      { id: 'service:blog_api', type: 'service', name: 'blog_api', stack: 'blog', status: 'running' },
    ];
    expect(filterGraphByStack(nodes, [], 'shop').nodes.map((n) => n.id)).toEqual(['swarmnode:mgr', 'service:shop_web']);
    expect(filterGraphByStack(nodes, [], '_standalone').nodes.map((n) => n.id)).toEqual(['swarmnode:mgr']);
    expect(listStacks(nodes).map((s) => s.name)).toEqual(['blog', 'shop']);
  });
});

describe('consumerStats', () => {
  const id = 'dvikfv6mt2qtwncfvburtsurx'; // 25-char swarm ID
  const node = 'xqp09okluv16vg9we4allmf4g';
  const s = {} as ContainerStatsData;

  it('recognises swarm task container names', () => {
    expect(serviceOfTask(`shop_web.1.${id}`)).toBe('shop_web');
    expect(serviceOfTask(`shop_web.12.${id}`)).toBe('shop_web');
    expect(serviceOfTask(`shop_agent.${node}.${id}`)).toBe('shop_agent');
    expect(serviceOfTask('shop_web')).toBeUndefined();
    expect(serviceOfTask('shop_web.backup')).toBeUndefined();
    expect(serviceOfTask('shop_web.1.backup')).toBeUndefined();
    expect(serviceOfTask(`shop_web.1.${id}.old`)).toBeUndefined();
    expect(serviceOfTask(`shop.api.1.${id}`)).toBe('shop.api');
    expect(serviceOfTask(`shop.api.${node}.${id}`)).toBe('shop.api');
  });

  it('counts each workload once', () => {
    const m = new Map([
      ['shop_web', s], // service aggregate
      [`shop_web.1.${id}`, s], // its tasks: covered by the aggregate
      [`shop_web.2.${id}`, s],
      [`orphan_svc.1.${id}`, s], // task without an aggregate: kept
      ['shop_web.backup', s], // standalone container with a dotted name: kept
      ['db', s],
      ['node:mgr', s], // node aggregate: dropped
      ['shop.api', s], // dotted service aggregate
      [`shop.api.1.${id}`, s], // its task: covered by the aggregate
    ]);
    expect([...consumerStats(m).keys()]).toEqual(['shop_web', `orphan_svc.1.${id}`, 'shop_web.backup', 'db', 'shop.api']);
  });

  it('drops history series covered by their service aggregate', () => {
    const series = {
      shop_web: 1,
      [`shop_web.1.${id}`]: 2,
      [`shop_agent.${node}.${id}`]: 3, // global task without an aggregate: kept
      'shop_web.backup': 4,
    };
    expect(withoutCoveredTasks(series)).toEqual({ shop_web: 1, [`shop_agent.${node}.${id}`]: 3, 'shop_web.backup': 4 });
  });
});

describe('showsSwarmWorkloads', () => {
  const svc: DGNode = { id: 'service:shop_web', type: 'service', name: 'shop_web', status: 'running' };
  const lone: DGNode = { id: 'container:lone', type: 'container', name: 'lone', status: 'running' };

  it('summarises services in swarm mode, for all stacks or a named one', () => {
    expect(showsSwarmWorkloads([svc], true, null)).toBe(true);
    expect(showsSwarmWorkloads([], true, 'shop')).toBe(true);
  });

  it('summarises containers in the standalone scope, even in swarm mode', () => {
    expect(showsSwarmWorkloads([lone], true, STANDALONE_STACK)).toBe(false);
    expect(showsSwarmWorkloads([], true, STANDALONE_STACK)).toBe(false);
  });

  it('summarises the containers of a Compose project on a swarm-mode daemon', () => {
    const api: DGNode = { id: 'container:blog-api-1', type: 'container', name: 'blog-api-1', status: 'exited', stack: 'blog' };
    expect(showsSwarmWorkloads([api], true, 'blog')).toBe(false);
  });

  it('counts service nodes as swarm before the system info has loaded', () => {
    expect(showsSwarmWorkloads([svc, lone], false, null)).toBe(true);
    expect(showsSwarmWorkloads([lone], false, null)).toBe(false);
  });
});

describe('isServiceLive', () => {
  const svc = (status: string, running?: number): DGNode => ({
    id: 'service:s', type: 'service', name: 's', status,
    ...(running === undefined ? {} : { service: { replicas: { running, desired: 2 } } }),
  });

  it('is live while a replica runs, whatever the status', () => {
    expect(isServiceLive(svc('degraded', 1))).toBe(true);
    expect(isServiceLive(svc('updating', 2))).toBe(true);
  });

  it('is not live with no replica running, even when degraded or updating', () => {
    expect(isServiceLive(svc('degraded', 0))).toBe(false);
    expect(isServiceLive(svc('updating', 0))).toBe(false);
  });

  it('falls back to a running status without replica counts', () => {
    expect(isServiceLive(svc('running'))).toBe(true);
    expect(isServiceLive(svc('degraded'))).toBe(false);
  });
});
