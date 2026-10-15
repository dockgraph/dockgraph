// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { ThemeProvider } from '../../theme';
import { DetailPanelSwarmNode } from './DetailPanelSwarmNode';
import type { DGNode } from '../../types';

afterEach(() => cleanup());

const node: DGNode = {
  id: 'swarmnode:w1', type: 'swarmnode', name: 'w1', status: 'ready',
  swarmNode: { id: 'w1', role: 'worker', availability: 'active', state: 'ready' },
};
const web: DGNode = {
  id: 'service:shop_web', type: 'service', name: 'shop_web', status: 'running',
  service: {
    mode: 'replicated', replicas: { running: 1, desired: 1 },
    tasks: [{ id: 't1', slot: 1, nodeId: 'w1', nodeHostname: 'w1', state: 'running', desiredState: 'running' }],
  },
};

describe('DetailPanelSwarmNode', () => {
  it('links each service on the node with a real, focusable button', () => {
    const onNavigate = vi.fn();
    render(
      <ThemeProvider>
        <DetailPanelSwarmNode node={node} dgNodes={[node, web]} statsMap={new Map()} onNavigate={onNavigate} />
      </ThemeProvider>,
    );
    // A <button>, so keyboard users can focus it and activate it with Enter/Space.
    const link = screen.getByRole('button', { name: 'shop_web' });
    link.focus();
    expect(document.activeElement).toBe(link);
    fireEvent.click(link);
    expect(onNavigate).toHaveBeenCalledWith('service:shop_web');
  });
});
