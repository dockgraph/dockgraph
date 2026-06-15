// @vitest-environment jsdom
import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { DetailPanelMounts } from './DetailPanelMounts';
import { ThemeProvider } from '../../theme';
import type { Mount } from '../../types/stats';

function renderMounts(mounts: Mount[], onNavigate?: (id: string) => void) {
  return render(
    <ThemeProvider>
      <DetailPanelMounts mounts={mounts} onNavigate={onNavigate} />
    </ThemeProvider>,
  );
}

describe('DetailPanelMounts', () => {
  it('renders nothing when there are no mounts', () => {
    const { container } = renderMounts([]);
    expect(container.innerHTML).toBe('');
  });

  it('links a named volume to its graph node by full name', () => {
    const onNavigate = vi.fn();
    renderMounts(
      [{ type: 'volume', name: 'demo-small_db_data', source: 'db_data', destination: '/var/lib/postgresql/data', rw: true }],
      onNavigate,
    );

    const link = screen.getByText('demo-small_db_data');
    fireEvent.click(link);
    expect(onNavigate).toHaveBeenCalledWith('volume:demo-small_db_data');
  });

  it('shows the host path for bind mounts and does not link them', () => {
    const onNavigate = vi.fn();
    renderMounts(
      [{ type: 'bind', source: '/etc/app/config', destination: '/config', rw: false }],
      onNavigate,
    );

    fireEvent.click(screen.getByText('/etc/app/config'));
    expect(onNavigate).not.toHaveBeenCalled();
  });
});
