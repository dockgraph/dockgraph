// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';

afterEach(() => cleanup());

vi.mock('@xyflow/react', () => ({
  Handle: () => null,
  Position: { Top: 'top', Bottom: 'bottom' },
  useStore: vi.fn(() => false),
}));

import { useStore } from '@xyflow/react';
import { ThemeProvider } from '../theme';
import { ServiceNode } from './ServiceNode';
import { STATUS_COLORS } from '../utils/colors';
import type { DGNode, ServiceNodeData } from '../types';
import type { NodeProps } from '@xyflow/react';

const baseNode: DGNode = {
  id: 'service:shop_web',
  type: 'service',
  name: 'shop_web',
  image: 'nginx:1.27',
  status: 'running',
  stack: 'shop',
  ports: [{ host: 8080, container: 80 }],
  service: { mode: 'replicated', replicas: { running: 3, desired: 3 } },
};

function renderNode(data: Partial<ServiceNodeData> = {}) {
  const props = { data: { dgNode: baseNode, nodeWidth: 200, ...data } } as unknown as NodeProps;
  return render(
    <ThemeProvider>
      <ServiceNode {...props} />
    </ThemeProvider>,
  );
}

describe('ServiceNode', () => {
  it('renders the name, image, ports, replicas badge and mode chip', () => {
    renderNode();
    expect(screen.getByText('shop_web')).toBeDefined();
    expect(screen.getByText('nginx:1.27')).toBeDefined();
    expect(screen.getByTestId('replicas-badge').textContent).toBe('3/3');
    expect(screen.getByTestId('mode-chip').textContent).toBe('repl');
    expect(screen.getByText(/8080/)).toBeDefined();
  });

  it('shows a global chip for global services', () => {
    renderNode({ dgNode: { ...baseNode, service: { mode: 'global', replicas: { running: 2, desired: 2 } } } });
    expect(screen.getByTestId('mode-chip').textContent).toBe('global');
  });

  it('colours a degraded service amber', () => {
    renderNode({ dgNode: { ...baseNode, status: 'degraded', service: { mode: 'replicated', replicas: { running: 1, desired: 3 } } } });
    const badge = screen.getByTestId('replicas-badge');
    expect(badge.textContent).toBe('1/3');
    expect(badge.style.color).toBe(hexToRgb(STATUS_COLORS.degraded));
    expect(badge.title).toContain('Degraded');
  });

  it('calls onInfoClick with the node id', () => {
    const onInfoClick = vi.fn();
    renderNode({ onInfoClick });
    fireEvent.click(screen.getByRole('button', { name: 'Inspect shop_web' }));
    expect(onInfoClick).toHaveBeenCalledWith('service:shop_web');
  });

  it('renders a compact name + badge at low zoom', () => {
    vi.mocked(useStore).mockReturnValueOnce(true);
    renderNode();
    expect(screen.getByText('shop_web')).toBeDefined();
    expect(screen.getByTestId('replicas-badge')).toBeDefined();
    expect(screen.queryByText('nginx:1.27')).toBeNull();
  });
});

function hexToRgb(hex: string): string {
  const n = parseInt(hex.slice(1), 16);
  return `rgb(${(n >> 16) & 255}, ${(n >> 8) & 255}, ${n & 255})`;
}
