// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { ThemeProvider } from '../../theme';
import { DetailPanelService, DetailPanelServiceHeader } from './DetailPanelService';
import type { ContainerStatsData, ServiceDetail } from '../../types/stats';

beforeEach(() => {
  vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(new Response(JSON.stringify({ lines: [] })))));
});
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

const detail: ServiceDetail = {
  id: 'svc123',
  name: 'shop_web',
  stack: 'shop',
  status: 'degraded',
  mode: 'replicated',
  replicas: { running: 1, desired: 2 },
  tasks: [
    { id: 'task1', slot: 1, nodeId: 'node-a', nodeHostname: 'worker-1', state: 'running', desiredState: 'running' },
    { id: 'task2', slot: 2, nodeId: 'node-b', nodeHostname: 'worker-2', state: 'failed', desiredState: 'shutdown', error: 'task: non-zero exit (1)' },
  ],
  labels: { 'com.docker.stack.namespace': 'shop' },
  ports: [{ host: 8080, container: 80, protocol: 'tcp', publishMode: 'ingress' }],
  networks: [{ name: 'shop_front', aliases: ['web'] }],
  image: 'nginx:1.27',
  env: [{ key: 'MODE', value: 'prod' }],
  mounts: [{ type: 'volume', name: 'shop_data', source: 'shop_data', destination: '/data', rw: true }],
  updateStatus: { state: 'rollback_completed', message: 'rollback done' },
};

const stats: ContainerStatsData = {
  cpuPercent: 12.5, cpuThrottled: 0, memUsage: 1048576, memLimit: 4194304,
  netRx: 0, netTx: 0, netRxErrors: 0, netTxErrors: 0, blockRead: 0, blockWrite: 0, pids: 3,
};

function renderPanel(statsMap = new Map<string, ContainerStatsData>(), onNavigate = vi.fn()) {
  render(
    <ThemeProvider>
      <DetailPanelService detail={detail} statsMap={statsMap} active={false} onNavigate={onNavigate} />
    </ThemeProvider>,
  );
  return onNavigate;
}

describe('DetailPanelServiceHeader', () => {
  it('shows name, image, status and mode/stack', () => {
    render(<ThemeProvider><DetailPanelServiceHeader detail={detail} /></ThemeProvider>);
    expect(screen.getByText('shop_web')).toBeDefined();
    expect(screen.getByText('nginx:1.27')).toBeDefined();
    expect(screen.getByText('Degraded')).toBeDefined();
    expect(screen.getByText(/replicated · stack shop/)).toBeDefined();
  });
});

describe('DetailPanelService', () => {
  it('says so when swarm could not report the replicas and tasks', () => {
    const unknown: ServiceDetail = { ...detail, status: 'unknown', replicas: null, tasks: [], tasksUnavailable: true };
    render(
      <ThemeProvider>
        <DetailPanelServiceHeader detail={unknown} />
        <DetailPanelService detail={unknown} statsMap={new Map()} active={false} onNavigate={vi.fn()} />
      </ThemeProvider>,
    );
    expect(screen.getAllByText('Unknown').length).toBe(2); // status badge and replicas row
    expect(screen.getByText('Tasks unavailable.')).toBeDefined();
    expect(screen.queryByText('No tasks.')).toBeNull();
  });

  it('shows replicas and update status', () => {
    renderPanel();
    expect(screen.getByText('1 / 2')).toBeDefined();
    expect(screen.getByText('rollback_completed — rollback done')).toBeDefined();
  });

  it('lists tasks with slot, node hostname, state and error', () => {
    renderPanel();
    expect(screen.getByText('Tasks (2)')).toBeDefined();
    expect(screen.getByText('worker-1')).toBeDefined();
    expect(screen.getByText('worker-2')).toBeDefined();
    expect(screen.getByText('failed')).toBeDefined();
    expect(screen.getByText('task: non-zero exit (1)')).toBeDefined();
  });

  it('shows per-task stats keyed by task container name', () => {
    renderPanel(new Map([['shop_web.1.task1', stats]]));
    expect(screen.getByText('12.5% · 1.0 MB')).toBeDefined();
  });

  it('reuses the ports, mounts, env and labels sections', () => {
    renderPanel();
    expect(screen.getByText('Ports')).toBeDefined();
    expect(screen.getByText('8080')).toBeDefined();
    expect(screen.getByText('Mounts')).toBeDefined();
    expect(screen.getByText('Environment')).toBeDefined();
    expect(screen.getByText('Labels')).toBeDefined();
  });

  it('navigates to networks and volumes', () => {
    const onNavigate = renderPanel();
    // A real button, so keyboard users can focus and activate it.
    fireEvent.click(screen.getByRole('button', { name: 'shop_front' }));
    expect(onNavigate).toHaveBeenCalledWith('network:shop_front');
    fireEvent.click(screen.getByText('shop_data'));
    expect(onNavigate).toHaveBeenCalledWith('volume:shop_data');
  });

  it('shows the empty task state', () => {
    render(
      <ThemeProvider>
        <DetailPanelService detail={{ ...detail, tasks: [] }} statsMap={new Map()} active={false} onNavigate={vi.fn()} />
      </ThemeProvider>,
    );
    expect(screen.getByText('No tasks.')).toBeDefined();
  });
});
