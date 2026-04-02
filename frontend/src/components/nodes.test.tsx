// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { render, screen, cleanup, fireEvent, waitFor } from '@testing-library/react';

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
import { InspectButton } from './InspectButton';
import { StatsMini } from './StatsMini';
import { LogoutButton } from './LogoutButton';
import type { ContainerStatsData } from '../types/stats';

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

// --- InspectButton ---

describe('InspectButton', () => {
  const defaultProps = {
    label: 'Inspect container',
    title: 'Open details',
    color: '#94a3b8',
    onClick: vi.fn(),
  };

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('renders three bars', () => {
    const { container } = render(<InspectButton {...defaultProps} />);
    const bars = container.querySelectorAll('span');
    expect(bars.length).toBe(3);
  });

  it('calls onClick when clicked', () => {
    const onClick = vi.fn();
    render(<InspectButton {...defaultProps} onClick={onClick} />);
    fireEvent.click(screen.getByRole('button'));
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it('stops event propagation on click', () => {
    const outerClick = vi.fn();
    const { container } = render(
      <div onClick={outerClick}>
        <InspectButton {...defaultProps} />
      </div>,
    );
    const button = container.querySelector('button')!;
    fireEvent.click(button);
    expect(defaultProps.onClick).toHaveBeenCalled();
    expect(outerClick).not.toHaveBeenCalled();
  });

  it('applies color prop to bars', () => {
    const { container } = render(<InspectButton {...defaultProps} color="#ff0000" />);
    const bars = container.querySelectorAll('span');
    bars.forEach((bar) => {
      expect(bar.style.background).toBe('rgb(255, 0, 0)');
    });
  });

  it('has correct aria-label', () => {
    render(<InspectButton {...defaultProps} label="Inspect web-app" />);
    expect(screen.getByRole('button', { name: 'Inspect web-app' })).toBeDefined();
  });
});

// --- StatsMini ---

describe('StatsMini', () => {
  function makeStats(overrides: Partial<ContainerStatsData> = {}): ContainerStatsData {
    return {
      cpuPercent: 25,
      cpuThrottled: 0,
      memUsage: 128 * 1024 * 1024,
      memLimit: 512 * 1024 * 1024,
      netRx: 0,
      netTx: 0,
      netRxErrors: 0,
      netTxErrors: 0,
      blockRead: 0,
      blockWrite: 0,
      pids: 5,
      ...overrides,
    };
  }

  it('returns null when stats is undefined', () => {
    const { container } = render(<StatsMini stats={undefined} />);
    expect(container.innerHTML).toBe('');
  });

  it('shows CPU percentage text', () => {
    render(<StatsMini stats={makeStats({ cpuPercent: 42.7 })} />);
    expect(screen.getByText(/43%/)).toBeDefined();
  });

  it('shows formatted memory text', () => {
    render(<StatsMini stats={makeStats({ memUsage: 128 * 1024 * 1024 })} />);
    expect(screen.getByText(/128M/)).toBeDefined();
  });

  it('uses green bar color when CPU < 60% and no throttle', () => {
    const { container } = render(<StatsMini stats={makeStats({ cpuPercent: 30, cpuThrottled: 0 })} />);
    const outerBar = container.querySelector('div[title]')!;
    const innerBar = outerBar.querySelector('div')!;
    // jsdom converts hex to rgb
    expect(innerBar.style.background).toBe('rgb(34, 197, 94)');
  });

  it('uses amber bar color when CPU >= 60%', () => {
    const { container } = render(<StatsMini stats={makeStats({ cpuPercent: 60, cpuThrottled: 0 })} />);
    const outerBar = container.querySelector('div[title]')!;
    const innerBar = outerBar.querySelector('div')!;
    expect(innerBar.style.background).toBe('rgb(245, 158, 11)');
  });

  it('uses amber bar color when throttle > 0 (even if CPU is low)', () => {
    const { container } = render(<StatsMini stats={makeStats({ cpuPercent: 10, cpuThrottled: 5 })} />);
    const outerBar = container.querySelector('div[title]')!;
    const innerBar = outerBar.querySelector('div')!;
    // Throttled containers use a striped gradient pattern with the amber color
    expect(innerBar.style.background).toContain('rgb(245, 158, 11)');
  });

  it('uses red bar color when CPU >= 85%', () => {
    const { container } = render(<StatsMini stats={makeStats({ cpuPercent: 90, cpuThrottled: 0 })} />);
    const outerBar = container.querySelector('div[title]')!;
    const innerBar = outerBar.querySelector('div')!;
    expect(innerBar.style.background).toBe('rgb(239, 68, 68)');
  });

  it('uses red bar color when throttle >= 50%', () => {
    const { container } = render(<StatsMini stats={makeStats({ cpuPercent: 10, cpuThrottled: 50 })} />);
    const outerBar = container.querySelector('div[title]')!;
    const innerBar = outerBar.querySelector('div')!;
    // Throttled containers use a striped gradient pattern with the red color
    expect(innerBar.style.background).toContain('rgb(239, 68, 68)');
  });

  it('clamps CPU bar width at 100% for values > 100', () => {
    const { container } = render(<StatsMini stats={makeStats({ cpuPercent: 150 })} />);
    const outerBar = container.querySelector('div[title]')!;
    const innerBar = outerBar.querySelector('div')!;
    expect(innerBar.style.width).toBe('100%');
  });

  it('shows throttle info in title when throttled', () => {
    const { container } = render(<StatsMini stats={makeStats({ cpuPercent: 40, cpuThrottled: 12 })} />);
    const barEl = container.querySelector('div[title]')!;
    expect(barEl.getAttribute('title')).toContain('throttled 12%');
  });

  it('hides memory when memUsage is 0', () => {
    render(<StatsMini stats={makeStats({ memUsage: 0 })} />);
    const text = screen.getByText(/25%/);
    expect(text.textContent).not.toContain('\u00b7');
  });
});

// --- LogoutButton (additional) ---

describe('LogoutButton', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('shows button when auth check returns JSON content-type', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response('{"authenticated":true}', {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    );
    render(<LogoutButton />);
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Sign out' })).toBeDefined();
    });
  });

  it('hides button when auth check returns HTML content-type (SPA fallback)', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response('<html></html>', {
        status: 200,
        headers: { 'Content-Type': 'text/html' },
      }),
    );
    const { container } = render(<LogoutButton />);
    await waitFor(() => {
      expect(container.innerHTML).toBe('');
    });
  });

  it('hides button when auth check has a network error', async () => {
    vi.spyOn(globalThis, 'fetch').mockRejectedValue(new Error('Network error'));
    const { container } = render(<LogoutButton />);
    await waitFor(() => {
      expect(container.innerHTML).toBe('');
    });
  });
});
