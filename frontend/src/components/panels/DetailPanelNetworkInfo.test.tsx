// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { ThemeProvider } from '../../theme';
import { DetailPanelNetworkInfo } from './DetailPanelNetworkInfo';
import type { NetworkDetail } from '../../types/stats';

afterEach(() => cleanup());

const base: NetworkDetail = {
  name: 'platform_ops',
  id: 'net1234567890abcdef',
  driver: 'overlay',
  scope: 'swarm',
  internal: false,
  enableIPv6: false,
  created: '',
  ipam: { driver: 'default', config: [] },
  options: {},
  labels: {},
  containers: [],
};

const swarmNetwork: NetworkDetail = {
  ...base,
  services: [
    {
      name: 'platform_log-agent',
      mode: 'global',
      vip: '10.0.9.38/24',
      tasks: [{ id: 't3', nodeId: 'n1', nodeHostname: 'dgc-m1', state: 'running', addresses: ['10.0.9.67/24'] }],
    },
    {
      name: 'platform_vault',
      mode: 'replicated',
      vip: '10.0.9.11/24',
      tasks: [
        { id: 't1', slot: 1, nodeId: 'n1', nodeHostname: 'dgc-m1', state: 'running', addresses: ['10.0.9.12/24'] },
        { id: 't2', slot: 2, nodeId: 'n2', nodeHostname: 'dgc-w7', state: 'running', addresses: ['10.0.9.13/24'] },
      ],
    },
  ],
};

function renderPanel(network: NetworkDetail) {
  const onNavigate = vi.fn();
  render(
    <ThemeProvider>
      <DetailPanelNetworkInfo network={network} onNavigate={onNavigate} />
    </ThemeProvider>,
  );
  return onNavigate;
}

describe('DetailPanelNetworkInfo swarm services', () => {
  it('lists the services by name with their VIP and task count, tasks folded', () => {
    renderPanel(swarmNetwork);
    expect(screen.getByText('Connected Services (2)')).toBeTruthy();
    expect(screen.getByRole('button', { name: 'platform_vault' })).toBeTruthy();
    expect(screen.getByText('10.0.9.11/24')).toBeTruthy();
    const toggle = screen.getByRole('button', { name: '2 tasks of platform_vault' });
    expect(toggle.textContent).toContain('2 tasks');
    expect(toggle.getAttribute('aria-expanded')).toBe('false');
    expect(screen.queryByText('10.0.9.12/24')).toBeNull();
  });

  it('unfolds a service into its tasks: slot, node and address', () => {
    renderPanel(swarmNetwork);
    fireEvent.click(screen.getByRole('button', { name: '2 tasks of platform_vault' }));
    expect(screen.getByRole('columnheader', { name: 'Slot' })).toBeTruthy();
    const row = screen.getByText('dgc-w7').closest('tr')!;
    expect(row.textContent).toContain('2');
    expect(row.textContent).toContain('10.0.9.13/24');
  });

  it('leaves the slot out for a global service, whose tasks have none', () => {
    renderPanel(swarmNetwork);
    fireEvent.click(screen.getByRole('button', { name: '1 task of platform_log-agent' }));
    expect(screen.getByText('10.0.9.67/24')).toBeTruthy();
    expect(screen.queryByRole('columnheader', { name: 'Slot' })).toBeNull();
  });

  it('opens a service from its name, as on the graph', () => {
    const onNavigate = renderPanel(swarmNetwork);
    fireEvent.click(screen.getByRole('button', { name: 'platform_vault' }));
    expect(onNavigate).toHaveBeenCalledWith('service:platform_vault');
  });

  it("says so when the swarm couldn't list the services, and shows this node's containers", () => {
    renderPanel({
      ...base,
      servicesUnavailable: true,
      containers: [{ name: 'platform_vault.1.kapyrtj2wg6y', ipv4Address: '10.0.9.12/24', ipv6Address: '', macAddress: '' }],
    });
    expect(screen.getByText(/couldn't be listed/)).toBeTruthy();
    expect(screen.getByText('platform_vault.1.kapyrtj2wg6y')).toBeTruthy();
  });

  it('has no services section for a network outside the swarm', () => {
    renderPanel({
      ...base,
      driver: 'bridge',
      scope: 'local',
      containers: [{ name: 'web', ipv4Address: '172.18.0.2/16', ipv6Address: '', macAddress: '' }],
    });
    expect(screen.queryByText(/Connected Services/)).toBeNull();
    expect(screen.getByText('Connected Containers')).toBeTruthy();
    expect(screen.getByText('web')).toBeTruthy();
  });
});
