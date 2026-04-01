// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/react';

afterEach(() => cleanup());

// Mock React Flow internals — components use Handle and useStore
vi.mock('@xyflow/react', () => ({
  Handle: () => null,
  Position: { Top: 'top', Bottom: 'bottom' },
  useStore: vi.fn(() => false),
}));

// Mock theme provider — all node components call useTheme()
const darkTheme = {
  mode: 'dark' as const,
  canvasBg: '#0f172a',
  dotColor: '#2d3a4d',
  nodeBg: '#1e293b',
  nodeBorder: '#334155',
  nodeGhostBorder: '#475569',
  nodeText: '#e2e8f0',
  nodeSubtext: '#64748b',
  portBg: '#0f172a',
  portText: '#94a3b8',
  edgeStroke: '#475569',
  panelBg: '#1e293b',
  panelBorder: '#334155',
  panelText: '#94a3b8',
  minimapBg: '#1e293b',
  minimapMask: 'rgba(15, 23, 42, 0.7)',
  groupBgAlpha: '0a',
  groupBorderAlpha: '4a',
  groupTextAlpha: 'd4',
};

vi.mock('../theme', () => ({
  useTheme: () => ({ theme: darkTheme, toggle: vi.fn() }),
}));

// Import after mocks are set up
import { ContainerNode } from './ContainerNode';
import { VolumeNode } from './VolumeNode';
import { NetworkGroup } from './NetworkGroup';
import { StatusIndicator } from './StatusIndicator';
import { ThemeToggle } from './ThemeToggle';

// --- ContainerNode ---

describe('ContainerNode', () => {
  const baseData = {
    dgNode: { id: 'c1', name: 'web-server', type: 'container', status: 'running', image: 'nginx:latest', ports: [] },
    nodeWidth: 200,
  };

  function renderContainer(overrides: Record<string, unknown> = {}) {
    const data = { ...baseData, dgNode: { ...baseData.dgNode, ...overrides } };
    return render(<ContainerNode id="c1" data={data} type="containerNode" />);
  }

  it('renders name, image, and accessible status dot', () => {
    renderContainer();
    expect(screen.getByText('web-server')).toBeDefined();
    expect(screen.getByText('nginx:latest')).toBeDefined();
    expect(screen.getByRole('img', { name: /running/i })).toBeDefined();
  });

  it('hides image section when image is not set', () => {
    renderContainer({ image: undefined });
    expect(screen.queryByTitle('nginx:latest')).toBeNull();
  });

  it('renders port mappings', () => {
    renderContainer({ ports: [{ host: '8080', container: '80' }] });
    expect(screen.getByText(/:8080/)).toBeDefined();
  });

  it('truncates ports beyond 3 and shows overflow count', () => {
    const ports = [
      { host: '80', container: '80' },
      { host: '443', container: '443' },
      { host: '8080', container: '8080' },
      { host: '9090', container: '9090' },
    ];
    renderContainer({ ports });
    expect(screen.getByText('+1')).toBeDefined();
  });

  it('uses dashed border for ghost (not_running) containers', () => {
    const { container } = renderContainer({ status: 'not_running' });
    const el = container.firstChild as HTMLElement;
    expect(el.style.borderTop).toContain('dashed');
  });

  it('applies reduced opacity for exited containers', () => {
    const { container } = renderContainer({ status: 'exited' });
    expect((container.firstChild as HTMLElement).style.opacity).toBe('0.5');
  });

  it('applies paused opacity for paused containers', () => {
    const { container } = renderContainer({ status: 'paused' });
    expect((container.firstChild as HTMLElement).style.opacity).toBe('0.7');
  });

  it('renders simplified view at low zoom', async () => {
    const rf = await import('@xyflow/react');
    vi.mocked(rf.useStore).mockImplementation(() => true); // isLowZoom = true

    const { container } = renderContainer();
    // Low zoom view doesn't render the image or status dot
    expect(container.querySelector('[role="img"]')).toBeNull();

    vi.mocked(rf.useStore).mockImplementation(() => false); // restore
  });
});

// --- VolumeNode ---

describe('VolumeNode', () => {
  const baseData = {
    dgNode: { id: 'v1', name: 'pgdata', type: 'volume', driver: 'local' },
    nodeWidth: 200,
  };

  it('renders the volume name', () => {
    render(<VolumeNode id="v1" data={baseData} type="volumeNode" />);
    expect(screen.getByText('pgdata')).toBeDefined();
  });

  it('renders the driver name', () => {
    render(<VolumeNode id="v1" data={baseData} type="volumeNode" />);
    expect(screen.getByText('local')).toBeDefined();
  });

  it('hides driver when not present', () => {
    const data = { ...baseData, dgNode: { ...baseData.dgNode, driver: undefined } };
    render(<VolumeNode id="v1" data={data} type="volumeNode" />);
    expect(screen.queryByText('local')).toBeNull();
  });

  it('applies ghost opacity for not_running volumes', () => {
    const data = { ...baseData, dgNode: { ...baseData.dgNode, status: 'not_running' } };
    const { container } = render(<VolumeNode id="v1" data={data} type="volumeNode" />);
    const style = (container.firstChild as HTMLElement).style;
    expect(style.opacity).toBe('0.5');
  });

  it('renders simplified block at low zoom', async () => {
    const rf = await import('@xyflow/react');
    vi.mocked(rf.useStore).mockImplementation(() => true);

    const { container } = render(<VolumeNode id="v1" data={baseData} type="volumeNode" />);
    // Low zoom view has no name text, just a colored block
    expect(screen.queryByText('pgdata')).toBeNull();
    expect((container.firstChild as HTMLElement).style.height).toBe('40px');

    vi.mocked(rf.useStore).mockImplementation(() => false);
  });
});

// --- NetworkGroup ---

describe('NetworkGroup', () => {
  it('renders the network name in uppercase', () => {
    const data = { dgNode: { id: 'n1', name: 'frontend', type: 'network' } };
    render(<NetworkGroup id="n1" data={data} type="networkGroup" />);
    expect(screen.getByText('frontend')).toBeDefined();
  });

  it('applies network color to border', () => {
    const data = { dgNode: { id: 'n1', name: 'backend', type: 'network' } };
    const { container } = render(<NetworkGroup id="n1" data={data} type="networkGroup" />);
    const style = (container.firstChild as HTMLElement).style;
    expect(style.border).toContain('solid');
  });
});

// --- StatusIndicator ---

describe('StatusIndicator', () => {
  it('shows Live when connected', () => {
    render(<StatusIndicator connected />);
    expect(screen.getByText('Live')).toBeDefined();
    expect(screen.getByRole('status')).toBeDefined();
  });

  it('shows Disconnected when not connected', () => {
    render(<StatusIndicator connected={false} />);
    expect(screen.getByText('Disconnected')).toBeDefined();
  });
});

// --- ThemeToggle ---

describe('ThemeToggle', () => {
  it('renders with theme toggle button', () => {
    render(<ThemeToggle />);
    expect(screen.getByRole('button', { name: /switch to light theme/i })).toBeDefined();
  });
});
