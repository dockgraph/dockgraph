import { describe, it, expect } from 'vitest';
import { Z, WINDOW_MIN_W, WINDOW_MIN_H, WINDOW_DEFAULT_W, WINDOW_DEFAULT_H, WINDOW_CASCADE, HEADER_HEIGHT, DOCK_HEIGHT } from './constants';

describe('layering + window constants', () => {
  it('orders the z-stack so windows sit above the panel but below header and dock', () => {
    expect(Z.detailPanel).toBeLessThan(Z.logWindowBase);
    expect(Z.logWindowBase).toBeLessThan(Z.dock);
    expect(Z.dock).toBeLessThan(Z.header);
  });

  it('defines sane window sizing defaults', () => {
    expect(WINDOW_MIN_W).toBeLessThanOrEqual(WINDOW_DEFAULT_W);
    expect(WINDOW_MIN_H).toBeLessThanOrEqual(WINDOW_DEFAULT_H);
    expect(WINDOW_CASCADE).toBeGreaterThan(0);
    expect(HEADER_HEIGHT).toBe(50);
    expect(DOCK_HEIGHT).toBeGreaterThan(0);
  });
});
