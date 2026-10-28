// @vitest-environment jsdom
import { describe, it, expect, afterEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/react';
import { ThemeProvider } from '../../theme';
import { DetailPanelVolume } from './DetailPanelVolume';
import type { VolumeDetail } from '../../types/stats';

afterEach(() => cleanup());

const volume: VolumeDetail = {
  name: 'shop_data', driver: 'local', mountpoint: '/var/lib/docker/volumes/shop_data/_data',
  scope: 'local', createdAt: '', labels: {}, options: {}, usageSize: 0, usageRefCount: 0, status: {},
};

function renderPanel(v: VolumeDetail) {
  render(
    <ThemeProvider>
      <DetailPanelVolume volume={v} />
    </ThemeProvider>,
  );
}

describe('DetailPanelVolume', () => {
  it('names the swarm node the details come from', () => {
    renderPanel({ ...volume, node: 'worker-2', nodes: ['worker-2'] });
    expect(screen.getByText('Node:')).toBeTruthy();
    expect(screen.getByText('worker-2')).toBeTruthy();
    expect(screen.queryByText('Also on:')).toBeNull();
  });

  it('lists the other nodes holding a volume of the same name', () => {
    renderPanel({ ...volume, node: 'manager-1', nodes: ['alpha', 'manager-1', 'worker-2'] });
    expect(screen.getByText('Also on:')).toBeTruthy();
    expect(screen.getByText('alpha, worker-2')).toBeTruthy();
  });

  it('shows no node outside a swarm', () => {
    renderPanel(volume);
    expect(screen.queryByText('Node:')).toBeNull();
  });
});
