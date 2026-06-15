// @vitest-environment jsdom
import { describe, it, expect, afterEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/react';

afterEach(() => cleanup());
import { DetailPanelCompose } from './DetailPanelCompose';
import { GhostContainerPanel } from './GhostContainerPanel';
import { ThemeProvider } from '../../theme';
import type { ComposeConfig, DGNode } from '../../types';

describe('DetailPanelCompose', () => {
  const compose: ComposeConfig = {
    service: 'worker',
    command: ['sh', '-c', 'sleep infinity'],
    environment: { DB_PASSWORD: '********', LOG_LEVEL: 'info' },
    labels: { 'com.example.team': 'platform' },
  };

  function renderCompose() {
    render(
      <ThemeProvider>
        <DetailPanelCompose compose={compose} />
      </ThemeProvider>,
    );
  }

  it('uses the "Process" section title (aligned with running containers)', () => {
    renderCompose();
    expect(screen.getByText('Process')).toBeDefined();
    expect(screen.queryByText('Service Configuration')).toBeNull();
  });

  it('masks sensitive env values and shows plain ones', () => {
    renderCompose();
    expect(screen.getByText('••••••••')).toBeDefined();
    expect(screen.getByText('info')).toBeDefined();
  });

  it('renders compose labels', () => {
    renderCompose();
    expect(screen.getByText('platform')).toBeDefined();
  });
});

describe('GhostContainerPanel ports', () => {
  it('shows the port protocol', () => {
    const node: DGNode = {
      id: 'container:demo-small-nginx-1',
      type: 'container',
      name: 'demo-small-nginx-1',
      status: 'not_running',
      ports: [{ host: 8070, container: 80, protocol: 'tcp' }],
    };
    render(
      <ThemeProvider>
        <GhostContainerPanel node={node} onNavigate={() => {}} />
      </ThemeProvider>,
    );
    expect(screen.getByText('80/tcp')).toBeDefined();
  });
});
