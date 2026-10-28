// @vitest-environment jsdom
import { describe, it, expect, afterEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/react';
import type { NodeProps } from '@xyflow/react';
import { ThemeProvider } from '../theme';
import { ControlSummary } from './ControlSummary';
import type { ControlSummaryData } from '../types';

afterEach(() => cleanup());

function renderSummary(data: ControlSummaryData) {
  const props = { data } as unknown as NodeProps;
  render(<ThemeProvider><ControlSummary {...props} /></ThemeProvider>);
  return screen.getByTestId('control-summary');
}

describe('ControlSummary', () => {
  // The counts are workers reporting ready; manager (raft) health isn't
  // measured, so the badge must not claim it.
  it('counts ready workers', () => {
    const badge = renderSummary({ ready: 12, total: 12, healthy: true });
    expect(badge.textContent).toBe('control / tcp 2377 / 12/12 workers ready');
    expect(badge.title).toBe('Workers reporting ready to the managers over the control plane (TCP 2377)');
  });

  it('counts down workers when any is down', () => {
    const badge = renderSummary({ ready: 10, total: 12, healthy: false });
    expect(badge.textContent).toBe('control / tcp 2377 / 2/12 workers down');
  });
});
