// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { cleanup, render, screen } from '@testing-library/react';
import type { EdgeProps } from '@xyflow/react';
import { ThemeProvider } from '../theme';
import { SwarmLinkEdge } from './SwarmLinkEdge';
import type { SwarmLinkData } from '../utils/swarmLinks';

// Two stacked nodes: the link runs from the bottom of `a` to the top of `b`.
const internal: Record<string, unknown> = {
  a: { internals: { positionAbsolute: { x: 0, y: 0 } }, measured: { width: 200, height: 100 } },
  b: { internals: { positionAbsolute: { x: 0, y: 300 } }, measured: { width: 200, height: 100 } },
};

vi.mock('@xyflow/react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@xyflow/react')>()),
  useInternalNode: (id: string) => internal[id],
  EdgeLabelRenderer: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

function renderEdge(data: SwarmLinkData, target = 'b') {
  const props = { id: 'e', source: 'a', target, data } as unknown as EdgeProps;
  return render(<ThemeProvider><svg><SwarmLinkEdge {...props} /></svg></ThemeProvider>);
}

describe('SwarmLinkEdge', () => {
  afterEach(cleanup);

  it('draws a healthy control spoke with a traveling arrow and no text label', () => {
    const { container } = renderEdge({ kind: 'control', healthy: true });
    expect(screen.queryByTestId('swarm-control-label')).toBeNull();
    // The arrowhead marker rides the path via animateMotion.
    expect(container.querySelector('animateMotion')).not.toBeNull();
    const d = container.querySelector('path')!.getAttribute('d')!;
    // Leaves a's bottom border (y=100) and enters b's top border (y=300).
    expect(d.startsWith('M100,100')).toBe(true);
    expect(d.endsWith('100,300')).toBe(true);
  });

  it('turns red, dashes, and labels the spoke "down" when unhealthy', () => {
    const { container } = renderEdge({ kind: 'control', healthy: false });
    expect(screen.getByTestId('swarm-control-label').textContent).toBe('control · 2377 · down');
    const path = container.querySelector('path') as SVGPathElement;
    expect(path.style.stroke).toBe('rgb(239, 68, 68)'); // STATUS_COLORS.exited
    expect(path.style.strokeDasharray).toBe('6 4');
    // No arrow marker on a down spoke.
    expect(container.querySelector('animateMotion')).toBeNull();
  });

  it('draws an overlay link with its networks as tooltip', () => {
    const { container } = renderEdge({ kind: 'overlay', networks: ['wp_back', 'wp_front'], stack: 'wp' });
    expect(screen.getByTestId('swarm-overlay-link')).toBeDefined();
    expect(container.querySelector('title')!.textContent).toBe('overlay: wp_back, wp_front');
    expect(screen.queryByTestId('swarm-control-label')).toBeNull();
  });

  it('renders nothing while an end node is unknown', () => {
    const { container } = renderEdge({ kind: 'control', healthy: true }, 'missing');
    expect(container.querySelector('path')).toBeNull();
  });
});
