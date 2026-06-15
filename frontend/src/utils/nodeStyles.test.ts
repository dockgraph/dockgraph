import { describe, it, expect } from 'vitest';
import { railColor } from './nodeStyles';
import { STATUS_COLORS, VOLUME_COLOR } from './colors';

describe('railColor', () => {
  it('uses the neutral not-running colour for ghost nodes', () => {
    expect(railColor(true, VOLUME_COLOR)).toBe(STATUS_COLORS.not_running);
  });

  it('uses the node accent colour when not a ghost', () => {
    expect(railColor(false, VOLUME_COLOR)).toBe(VOLUME_COLOR);
    expect(railColor(false, STATUS_COLORS.running)).toBe(STATUS_COLORS.running);
  });
});
