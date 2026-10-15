import { describe, it, expect } from 'vitest';
import { evaluateAlerts } from './alerts';
import type { DGNode } from '../types';
import type { ContainerStatsData } from '../types/stats';

function stats(over: Partial<ContainerStatsData>): ContainerStatsData {
  return {
    cpuPercent: 0, cpuThrottled: 0, memUsage: 0, memLimit: 0,
    netRx: 0, netTx: 0, netRxErrors: 0, netTxErrors: 0,
    blockRead: 0, blockWrite: 0, pids: 1, ...over,
  };
}

function service(name: string, status: string, running: number, desired: number): DGNode {
  return { id: `service:${name}`, type: 'service', name, status, service: { mode: 'replicated', replicas: { running, desired } } };
}

describe('evaluateAlerts', () => {
  it('alerts on exited and restarting containers, pointing at their node', () => {
    const alerts = evaluateAlerts([
      { id: 'container:db', type: 'container', name: 'db', status: 'exited' },
      { id: 'container:web', type: 'container', name: 'web', status: 'restarting' },
    ], new Map());
    expect(alerts).toEqual([
      { severity: 'error', nodeId: 'container:db', container: 'db', message: 'Container exited' },
      { severity: 'warning', nodeId: 'container:web', container: 'web', message: 'Container restarting' },
    ]);
  });

  it('alerts on a degraded swarm service, pointing at the service node', () => {
    const alerts = evaluateAlerts([service('shop_api', 'degraded', 1, 3), service('shop_web', 'running', 2, 2)], new Map());
    expect(alerts).toEqual([
      { severity: 'warning', nodeId: 'service:shop_api', container: 'shop_api', message: 'Service degraded: 1/3 running' },
    ]);
  });

  it('checks a service aggregate against the thresholds per replica', () => {
    const nodes = [service('hot', 'running', 2, 2), service('busy', 'running', 3, 3)];
    const map = new Map([
      ['hot', stats({ cpuPercent: 190, memUsage: 95, memLimit: 100 })], // 95% per replica
      ['busy', stats({ cpuPercent: 150 })], // 50% per replica: fine
    ]);
    const messages = evaluateAlerts(nodes, map).map((a) => `${a.container}: ${a.message}`);
    expect(messages).toEqual(['hot: High CPU: 95.0% per replica', 'hot: Memory usage > 90% of limit']);
  });

  it('ignores networks, volumes and swarm nodes', () => {
    const alerts = evaluateAlerts([
      { id: 'network:app', type: 'network', name: 'app', status: 'exited' },
      { id: 'swarmnode:n1', type: 'swarmnode', name: 'n1', status: 'down' },
    ], new Map([['app', stats({ cpuPercent: 99 })]]));
    expect(alerts).toEqual([]);
  });
});
