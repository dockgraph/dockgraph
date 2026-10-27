// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';

afterEach(() => cleanup());

vi.mock('@xyflow/react', () => ({
  Handle: () => null,
  Position: { Top: 'top', Bottom: 'bottom' },
  useStore: vi.fn(() => false),
}));

import { ThemeProvider } from '../theme';
import { SwarmNodeGroup } from './SwarmNodeGroup';
import { NodeServiceCard } from './NodeServiceCard';
import { RoleGroup } from './RoleGroup';
import { serviceCardHeight } from '../layout/nodeLayout';
import { INACTIVE_OPACITY } from '../utils/constants';
import type { DGNode, SwarmNodeGroupData, NodeServiceCardData, RoleGroupData } from '../types';
import type { ContainerStatsData } from '../types/stats';
import type { NodeProps } from '@xyflow/react';

const stats: ContainerStatsData = {
  cpuPercent: 200, cpuThrottled: 0, memUsage: 2 * 1024 ** 3, memLimit: 0,
  netRx: 0, netTx: 0, netRxErrors: 0, netTxErrors: 0, blockRead: 0, blockWrite: 0, pids: 0,
};

const mgr: DGNode = {
  id: 'swarmnode:mgr', type: 'swarmnode', name: 'mgr', status: 'ready',
  swarmNode: { id: 'n1', role: 'manager', leader: true, availability: 'active', state: 'ready', nanoCpus: 4e9, memoryBytes: 8 * 1024 ** 3 },
};

function renderGroup(data: Partial<SwarmNodeGroupData>) {
  const props = { data: { dgNode: mgr, taskCount: 3, ...data } } as unknown as NodeProps;
  return render(<ThemeProvider><SwarmNodeGroup {...props} /></ThemeProvider>);
}

describe('SwarmNodeGroup', () => {
  it('shows hostname, leader badge, task count and resource bars', () => {
    renderGroup({ stats });
    expect(screen.getByText('mgr')).toBeDefined();
    expect(screen.getByTestId('role-badge').textContent).toBe('manager ★');
    expect(screen.getByTestId('task-count').textContent).toBe('3 tasks');
    expect(screen.getByText('CPU')).toBeDefined();
    expect(screen.getByText('50%')).toBeDefined();
    expect(screen.queryByTestId('availability-chip')).toBeNull();
    expect(screen.queryByTestId('no-agent')).toBeNull();
  });

  it('shows a drain chip and "no agent" without stats', () => {
    renderGroup({
      dgNode: { ...mgr, swarmNode: { ...mgr.swarmNode!, role: 'worker', leader: false, availability: 'drain' } },
    });
    expect(screen.getByTestId('role-badge').textContent).toBe('worker');
    expect(screen.getByTestId('availability-chip').textContent).toBe('drain');
    expect(screen.getByTestId('no-agent').textContent).toContain('no agent');
  });

  it('highlights on a header click and opens the node panel only from ☰', () => {
    const onInfoClick = vi.fn();
    // Stands in for React Flow's node click, which toggles the highlight.
    const onNodeClick = vi.fn();
    const props = { data: { dgNode: mgr, taskCount: 3, onInfoClick } } as unknown as NodeProps;
    render(<ThemeProvider><div onClick={onNodeClick}><SwarmNodeGroup {...props} /></div></ThemeProvider>);

    fireEvent.click(screen.getByText('mgr'));
    expect(onNodeClick).toHaveBeenCalledTimes(1);
    expect(onInfoClick).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole('button', { name: 'Inspect mgr' }));
    expect(onInfoClick).toHaveBeenCalledWith('swarmnode:mgr');
    expect(onNodeClick).toHaveBeenCalledTimes(1);
  });

  it('renders the Unassigned box without an inspect action', () => {
    const onInfoClick = vi.fn();
    renderGroup({
      dgNode: { id: 'nodegroup:unassigned', type: 'swarmnode', name: 'Unassigned' },
      unassigned: true,
      taskCount: 1,
      onInfoClick,
    });
    fireEvent.click(screen.getByText('Unassigned'));
    expect(onInfoClick).not.toHaveBeenCalled();
    expect(screen.getByText('Tasks waiting for a node')).toBeDefined();
    expect(screen.queryByTestId('role-badge')).toBeNull();
  });
});

describe('SwarmNodeGroup states', () => {
  it('says "No tasks" for an empty node box', () => {
    renderGroup({ taskCount: 0, role: 'manager' });
    expect(screen.getByTestId('no-tasks').textContent).toBe('No tasks');
  });

  it('shows a state chip for a down node', () => {
    renderGroup({ dgNode: { ...mgr, swarmNode: { ...mgr.swarmNode!, state: 'down' } } });
    expect(screen.getByTestId('state-chip').textContent).toBe('down');
    expect((screen.getByTestId('swarm-node-group') as HTMLElement).style.opacity).toBe('0.7');
  });

  it('does not show a state chip for a ready node', () => {
    renderGroup({});
    expect(screen.queryByTestId('state-chip')).toBeNull();
    expect(screen.queryByTestId('no-tasks')).toBeNull();
  });
});

describe('RoleGroup', () => {
  function renderRole(data: RoleGroupData) {
    const props = { data } as unknown as NodeProps;
    return render(<ThemeProvider><RoleGroup {...props} /></ThemeProvider>);
  }

  it('shows the role title with node and task counts in a legend tab', () => {
    renderRole({ role: 'manager', nodeCount: 3, taskCount: 5 });
    expect(screen.getByTestId('role-group-tab').textContent).toContain('Managers');
    expect(screen.getByTestId('role-group-summary').textContent).toBe('3 nodes · 5 tasks');
    expect(screen.getByTestId('role-group').getAttribute('data-role')).toBe('manager');
  });

  it('is fixed in place like the network view: no grab cursor or drag hint', () => {
    renderRole({ role: 'worker', nodeCount: 2, taskCount: 3 });
    const group = screen.getByTestId('role-group') as HTMLElement;
    expect(group.style.cursor).toBe('');
    expect(group.title).toBe('Workers: 2 nodes, 3 tasks');
  });

  it('uses a distinct colour per role', () => {
    renderRole({ role: 'worker', nodeCount: 1, taskCount: 1 });
    const worker = (screen.getByTestId('role-group') as HTMLElement).style.borderColor;
    cleanup();
    renderRole({ role: 'manager', nodeCount: 1, taskCount: 1 });
    expect(screen.getByTestId('role-group-tab').textContent).toContain('Managers');
    expect((screen.getByTestId('role-group') as HTMLElement).style.borderColor).not.toBe(worker);
    expect(screen.getByTestId('role-group-summary').textContent).toBe('1 node · 1 task');
  });
});

describe('NodeServiceCard', () => {
  const base: NodeServiceCardData = {
    dgNode: { id: 'nodesvc:mgr:shop_web', type: 'service', name: 'shop_web', stack: 'shop' },
    serviceId: 'service:shop_web',
    serviceName: 'shop_web',
    stack: 'shop',
    tasks: [
      { id: 't1', slot: 1, state: 'running', desiredState: 'running' },
      { id: 't3', slot: 3, state: 'pending', desiredState: 'running', error: 'no suitable node' },
    ],
  };
  function renderCard(data: Partial<NodeServiceCardData>) {
    const props = { data: { ...base, ...data } } as unknown as NodeProps;
    return render(<ThemeProvider><NodeServiceCard {...props} /></ThemeProvider>);
  }

  it('shows the service, a running badge and one row per task', () => {
    renderCard({ taskStats: { t1: { ...stats, cpuPercent: 12 } } });
    expect(screen.getByText('shop_web')).toBeDefined();
    expect(screen.getByTestId('card-running-badge').textContent).toBe('1/2');
    const rows = screen.getAllByTestId('task-row');
    expect(rows).toHaveLength(2);
    expect(rows[0].textContent).toContain('.1');
    expect(rows[0].textContent).toMatch(/12%/);
    expect(rows[1].textContent).toContain('no suitable node');
  });

  it('labels a running replicated-job task in slot 0 by its slot, not as global', () => {
    renderCard({ mode: 'replicated-job', tasks: [{ id: 'j0', state: 'running', desiredState: 'complete' }] });
    const row = screen.getByTestId('task-row');
    expect(row.textContent).toContain('.0');
    expect(row.textContent).not.toContain('global');
    expect(row.title).toContain('shop_web.0');
  });

  it('labels global tasks and sizes the card to its rows', () => {
    renderCard({ tasks: [{ id: 'g', state: 'running', desiredState: 'running' }] });
    expect(screen.getByTestId('task-row').textContent).toContain('global');
    expect((screen.getByTestId('node-service-card') as HTMLElement).style.height).toBe(`${serviceCardHeight(1)}px`);
  });

  it('highlights on a card or task-row click and opens the service only from ☰', () => {
    const onInfoClick = vi.fn();
    // Stands in for React Flow's node click, which toggles the highlight.
    const onNodeClick = vi.fn();
    const props = { data: { ...base, onInfoClick } } as unknown as NodeProps;
    render(<ThemeProvider><div onClick={onNodeClick}><NodeServiceCard {...props} /></div></ThemeProvider>);

    fireEvent.click(screen.getByTestId('node-service-card'));
    fireEvent.click(screen.getAllByTestId('task-row')[1]);
    expect(onNodeClick).toHaveBeenCalledTimes(2);
    expect(onInfoClick).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole('button', { name: 'Inspect shop_web' }));
    expect(onInfoClick).toHaveBeenCalledWith('service:shop_web');
    expect(onNodeClick).toHaveBeenCalledTimes(2);
  });

  it('outlines peers on hover and dims cards on inactive nodes', () => {
    renderCard({ peerHover: true, nodeInactive: true });
    const card = screen.getByTestId('node-service-card') as HTMLElement;
    expect(card.getAttribute('data-peer-hover')).toBe('true');
    expect(card.style.opacity).toBe(String(INACTIVE_OPACITY));
  });
});
