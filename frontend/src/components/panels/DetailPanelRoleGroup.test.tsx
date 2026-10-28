// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup, within } from '@testing-library/react';
import { ThemeProvider } from '../../theme';
import { DetailPanelRoleGroup, DetailPanelRoleGroupHeader } from './DetailPanelRoleGroup';
import { summarizeRole } from '../../utils/roleSummary';
import { boxTaskCounts } from '../../utils/nodeTransform';
import { nodeStatsKey } from '../../utils/stack';
import type { DGNode, SwarmNodeInfo, SwarmRole } from '../../types';
import type { ContainerStatsData, SwarmManager } from '../../types/stats';

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

function swarmNode(name: string, info: Partial<SwarmNodeInfo>): DGNode {
  const swarmNode: SwarmNodeInfo = { id: `id-${name}`, role: 'manager', availability: 'active', state: 'ready', ...info };
  return { id: `swarmnode:${name}`, type: 'swarmnode', name, status: swarmNode.state, swarmNode };
}

const stats = (cpuPercent: number, memUsage: number): ContainerStatsData => ({
  cpuPercent, memUsage, cpuThrottled: 0, memLimit: 0, netRx: 0, netTx: 0,
  netRxErrors: 0, netTxErrors: 0, blockRead: 0, blockWrite: 0, pids: 0,
});

const graph: DGNode[] = [
  swarmNode('m1', { leader: true, engineVersion: '28.5.1', nanoCpus: 4e9, memoryBytes: 8e9 }),
  swarmNode('m2', { availability: 'drain', engineVersion: '28.5.1', nanoCpus: 2e9, memoryBytes: 4e9 }),
  swarmNode('m3', { state: 'down', engineVersion: '27.3.1', nanoCpus: 2e9, memoryBytes: 4e9 }),
  swarmNode('w1', { role: 'worker', engineVersion: '28.5.1' }),
  {
    id: 'service:shop_web', type: 'service', name: 'shop_web', status: 'running',
    service: {
      mode: 'replicated',
      replicas: { running: 2, desired: 2 },
      tasks: [
        { id: 't1', slot: 1, nodeHostname: 'm1', state: 'running', desiredState: 'running' },
        { id: 't2', slot: 2, nodeHostname: 'm2', state: 'running', desiredState: 'running' },
      ],
    },
  },
];
const statsMap = new Map([[nodeStatsKey('m1'), stats(200, 2e9)], [nodeStatsKey('m2'), stats(50, 1e9)]]);

/** Stubs /api/swarm/managers with one reply. */
function stubManagers(reply: { status: number; body: unknown }) {
  const fetchMock = vi.fn(() => Promise.resolve(new Response(JSON.stringify(reply.body), { status: reply.status })));
  vi.stubGlobal('fetch', fetchMock);
  return fetchMock;
}

const manager = (name: string, reachability: string, leader = false, id = `id-${name}`): SwarmManager => ({ id, name, reachability, leader });

const summaryOf = (role: SwarmRole) => summarizeRole(graph, role, statsMap, boxTaskCounts(graph, null));

function renderPanel(role: SwarmRole) {
  const onNavigate = vi.fn();
  render(
    <ThemeProvider>
      <DetailPanelRoleGroup role={role} summary={summaryOf(role)} onNavigate={onNavigate} />
    </ThemeProvider>,
  );
  return onNavigate;
}

/** The Nodes table's row of `name`. */
const nodeRow = (name: string) => within(section(/Nodes \(\d+\)/)).getByRole('button', { name }).closest('tr')!;

/** The section whose fold button has `title`. */
const section = (title: RegExp) => screen.getByRole('button', { name: title }).parentElement!;

describe('DetailPanelRoleGroup quorum', () => {
  it("reads the managers' raft state live and shows what losing one more would mean", async () => {
    stubManagers({ status: 200, body: { managers: [manager('m1', 'reachable', true), manager('m2', 'reachable'), manager('m3', 'unreachable')] } });
    const onNavigate = renderPanel('manager');

    const quorum = section(/Quorum/);
    expect(await within(quorum).findByText('degraded')).toBeTruthy();
    expect(within(quorum).getByText('2 of 3 managers')).toBeTruthy();
    expect(within(quorum).getByText('no more managers')).toBeTruthy();
    fireEvent.click(within(quorum).getByRole('button', { name: 'm3' }));
    expect(onNavigate).toHaveBeenCalledWith('swarmnode:m3');
  });

  it('is healthy with every manager reachable', async () => {
    stubManagers({ status: 200, body: { managers: [manager('m1', 'reachable', true), manager('m2', 'reachable'), manager('m3', 'reachable')] } });
    renderPanel('manager');
    const quorum = section(/Quorum/);
    expect(await within(quorum).findByText('healthy')).toBeTruthy();
    expect(within(quorum).getByText('1 more manager')).toBeTruthy();
  });

  it("says so when the managers can't answer, instead of the last known state", async () => {
    stubManagers({ status: 503, body: { error: 'the swarm has no leader: fewer than a majority of its managers are reachable' } });
    renderPanel('manager');
    const quorum = section(/Quorum/);
    expect(await within(quorum).findByText('unavailable')).toBeTruthy();
    expect(within(quorum).getByText(/the swarm has no leader/)).toBeTruthy();
  });

  it('notes that an even number of managers adds no fault tolerance', async () => {
    stubManagers({ status: 200, body: { managers: ['a', 'b', 'c', 'd'].map((n) => manager(n, 'reachable', n === 'a')) } });
    renderPanel('manager');
    expect(await screen.findByText(/4 tolerate as many failures as 3/)).toBeTruthy();
  });

  it('is not healthy without a leader, even with every manager reachable', async () => {
    stubManagers({ status: 200, body: { managers: [manager('m1', 'reachable'), manager('m2', 'reachable'), manager('m3', 'reachable')] } });
    renderPanel('manager');
    expect(await within(section(/Quorum/)).findByText('no leader')).toBeTruthy();
  });

  it('marks the leader the swarm reports now, not the one in the graph', async () => {
    // The graph still has m1 as leader; leadership has moved to m2.
    stubManagers({ status: 200, body: { managers: [manager('m2', 'reachable', true), manager('m1', 'reachable'), manager('m3', 'reachable')] } });
    renderPanel('manager');
    await within(section(/Quorum/)).findByText('healthy');
    expect(within(nodeRow('m2')).queryByTitle('Leader')).toBeTruthy();
    expect(within(nodeRow('m1')).queryByTitle('Leader')).toBeNull();
  });

  it("marks no leader while the managers can't answer", async () => {
    stubManagers({ status: 503, body: { error: 'the swarm has no leader: fewer than a majority of its managers are reachable' } });
    renderPanel('manager');
    await within(section(/Quorum/)).findByText('unavailable');
    expect(within(section(/Nodes \(3\)/)).queryByTitle('Leader')).toBeNull();
  });

  it("names a manager the graph doesn't show yet without linking to an empty panel", async () => {
    stubManagers({ status: 200, body: { managers: [manager('m1', 'reachable', true), manager('m2', 'reachable'), manager('m9', 'unreachable')] } });
    renderPanel('manager');
    const quorum = section(/Quorum/);
    expect(await within(quorum).findByText('m9')).toBeTruthy();
    expect(within(quorum).queryByRole('button', { name: 'm9' })).toBeNull();
  });

  it('lists every unreachable entry of a hostname a rejoined manager left behind', async () => {
    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {});
    stubManagers({
      status: 200,
      body: { managers: [manager('m1', 'reachable', true), manager('m3', 'unreachable', false, 'old'), manager('m3', 'unreachable', false, 'older')] },
    });
    renderPanel('manager');
    const quorum = section(/Quorum/);
    await within(quorum).findByText('quorum lost');
    expect(within(quorum).getAllByRole('button', { name: 'm3' })).toHaveLength(2);
    expect(consoleError).not.toHaveBeenCalled(); // no duplicate React keys
    consoleError.mockRestore();
  });

  it("doesn't ask for the managers in the workers panel", () => {
    const fetchMock = stubManagers({ status: 200, body: { managers: [] } });
    renderPanel('worker');
    expect(screen.queryByRole('button', { name: /Quorum/ })).toBeNull();
    expect(fetchMock).not.toHaveBeenCalled();
  });
});

describe('DetailPanelRoleGroup summary', () => {
  it('tallies the states and availability of the nodes', () => {
    stubManagers({ status: 200, body: { managers: [] } });
    renderPanel('manager');
    const status = section(/Status/);
    expect(within(status).getByText('2 ready · 1 down')).toBeTruthy();
    expect(within(status).getByText('2 active · 1 drain')).toBeTruthy();
  });

  it('shows usage over the nodes that report it, and says which', () => {
    stubManagers({ status: 200, body: { managers: [] } });
    renderPanel('manager');
    const resources = section(/Resources/);
    expect(within(resources).getByText(/8 cores/)).toBeTruthy();
    expect(within(resources).getByText(/2 of 3 nodes/)).toBeTruthy();
  });

  it('notes mixed engine versions', () => {
    stubManagers({ status: 200, body: { managers: [] } });
    renderPanel('manager');
    expect(within(section(/Engine/)).getByText(/different engine versions/)).toBeTruthy();
  });

  it('lists the nodes with their tasks, each opening its own panel', () => {
    stubManagers({ status: 200, body: { managers: [] } });
    const onNavigate = renderPanel('manager');
    const nodes = section(/Nodes \(3\)/);
    const rows = within(nodes).getAllByRole('row').slice(1);
    expect(rows.map((r) => within(r).getByRole('button').textContent)).toEqual(['m1', 'm2', 'm3']);
    expect(within(rows[0]).getByText('1')).toBeTruthy(); // one task on m1
    fireEvent.click(within(rows[1]).getByRole('button', { name: 'm2' }));
    expect(onNavigate).toHaveBeenCalledWith('swarmnode:m2');
  });
});

describe('DetailPanelRoleGroupHeader', () => {
  it('names the group with its node and task counts', () => {
    render(
      <ThemeProvider>
        <DetailPanelRoleGroupHeader role="manager" summary={summaryOf('manager')} />
      </ThemeProvider>,
    );
    expect(screen.getByText('Managers')).toBeTruthy();
    expect(screen.getByText('swarm managers · 3 nodes · 2 tasks')).toBeTruthy();
  });
});
