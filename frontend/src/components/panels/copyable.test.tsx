// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach, type Mock } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';

afterEach(() => cleanup());
import { Row } from './shared';
import { KeyValueList } from './KeyValueList';
import { GhostHeader } from './GhostHeader';
import { ThemeProvider, useTheme } from '../../theme';
import type { DGNode } from '../../types';

/** GhostHeader takes a theme prop; pull it from context to match real usage. */
function GhostHeaderHarness({ node }: { node: DGNode }) {
  const { theme } = useTheme();
  return <GhostHeader node={node} theme={theme} />;
}

function mockClipboard(): Mock {
  const writeText = vi.fn().mockResolvedValue(undefined);
  Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true });
  return writeText;
}

describe('panel click-to-copy', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('Row copies its value', () => {
    const writeText = mockClipboard();
    render(
      <ThemeProvider>
        <Row label="Subnet" value="172.18.0.0/16" mono={{}} subtext="#999" />
      </ThemeProvider>,
    );

    fireEvent.click(screen.getByText('172.18.0.0/16'));
    expect(writeText).toHaveBeenCalledWith('172.18.0.0/16');
  });

  it('KeyValueList copies a value without the key prefix', () => {
    const writeText = mockClipboard();
    render(
      <ThemeProvider>
        <KeyValueList entries={{ POSTGRES_PASSWORD: 'demo' }} />
      </ThemeProvider>,
    );

    fireEvent.click(screen.getByText('demo'));
    expect(writeText).toHaveBeenCalledWith('demo');
  });

  it('GhostHeader copies the node name and image', () => {
    const writeText = mockClipboard();
    const node: DGNode = { id: 'container:demo-small-worker-1', type: 'container', name: 'demo-small-worker-1', image: 'busybox', status: 'not_running' };
    render(
      <ThemeProvider>
        <GhostHeaderHarness node={node} />
      </ThemeProvider>,
    );

    fireEvent.click(screen.getByText('demo-small-worker-1'));
    expect(writeText).toHaveBeenCalledWith('demo-small-worker-1');

    fireEvent.click(screen.getByText('busybox'));
    expect(writeText).toHaveBeenCalledWith('busybox');
  });
});
