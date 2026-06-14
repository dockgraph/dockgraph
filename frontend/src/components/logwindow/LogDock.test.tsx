// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { ThemeProvider } from '../../theme';
import { LogDock } from './LogDock';
import type { LogWindowState } from '../../hooks/logWindowsState';
import type { ReactNode } from 'react';

afterEach(() => cleanup());

const mk = (id: string, title: string, tabs = 1): LogWindowState => ({
  id, activeTab: 0, x: 0, y: 0, w: 460, h: 340, z: 1, minimized: true,
  tabs: Array.from({ length: tabs }, (_, i) => ({ containerId: `${title}-${i}`, title, search: '' })),
});

const wrap = (ui: ReactNode) => render(<ThemeProvider>{ui}</ThemeProvider>);

describe('LogDock', () => {
  it('renders nothing when no windows are minimized', () => {
    const { container } = wrap(<LogDock minimized={[]} onRestore={vi.fn()} />);
    expect(container.firstChild).toBeNull();
  });

  it('renders a pill per minimized window and restores on click', () => {
    const onRestore = vi.fn();
    wrap(<LogDock minimized={[mk('lw-1', 'nginx-1'), mk('lw-2', 'redis-1', 3)]} onRestore={onRestore} />);
    expect(screen.getByText('nginx-1')).toBeTruthy();
    expect(screen.getByText(/·3 tabs/)).toBeTruthy();
    fireEvent.click(screen.getByText('nginx-1'));
    expect(onRestore).toHaveBeenCalledWith('lw-1');
  });
});
