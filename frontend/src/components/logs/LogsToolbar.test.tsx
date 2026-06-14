// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { ThemeProvider } from '../../theme';
import { LogsToolbar } from './LogsToolbar';
import { emptyFilters, withChip } from '../../hooks/useLogFilters';
import type { ReactNode } from 'react';

afterEach(() => cleanup());
const wrap = (ui: ReactNode) => render(<ThemeProvider>{ui}</ThemeProvider>);

const baseProps = {
  filters: emptyFilters,
  onText: vi.fn(),
  onRemoveChip: vi.fn(),
  onClear: vi.fn(),
  onToggleRegex: vi.fn(),
  paused: false,
  onTogglePause: vi.fn(),
  shown: 10,
  total: 42,
};

describe('LogsToolbar', () => {
  it('typing calls onText', () => {
    wrap(<LogsToolbar {...baseProps} />);
    fireEvent.change(screen.getByPlaceholderText('Filter logs'), { target: { value: 'req-1' } });
    expect(baseProps.onText).toHaveBeenCalledWith('req-1');
  });

  it('renders a removable chip and removes it', () => {
    const onRemoveChip = vi.fn();
    const filters = withChip(emptyFilters, { kind: 'exclude', container: 'db' });
    wrap(<LogsToolbar {...baseProps} filters={filters} onRemoveChip={onRemoveChip} />);
    expect(screen.getByText(/exclude: db/i)).toBeTruthy();
    fireEvent.click(screen.getByLabelText('Remove filter exclude: db'));
    expect(onRemoveChip).toHaveBeenCalledWith({ kind: 'exclude', container: 'db' });
  });

  it('shows the shown/total counts', () => {
    wrap(<LogsToolbar {...baseProps} />);
    expect(screen.getByText(/10 \/ 42/)).toBeTruthy();
  });

  it('toggles regex mode', () => {
    const onToggleRegex = vi.fn();
    wrap(<LogsToolbar {...baseProps} onToggleRegex={onToggleRegex} />);
    fireEvent.click(screen.getByLabelText('Use regex'));
    expect(onToggleRegex).toHaveBeenCalled();
  });
});
