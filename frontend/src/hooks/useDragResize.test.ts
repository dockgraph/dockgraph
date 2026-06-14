import { describe, it, expect } from 'vitest';
import { applyResize } from './useDragResize';

const start = { x: 100, y: 100, w: 460, h: 340 };
const MIN = { w: 280, h: 160 };

describe('applyResize', () => {
  it('grows from the right/bottom edge', () => {
    const r = applyResize(start, 'se', 50, 30, MIN);
    expect(r.w).toBe(510);
    expect(r.h).toBe(370);
    expect(r.x).toBe(100);
    expect(r.y).toBe(100);
  });

  it('dragging the left edge moves x and changes width', () => {
    const r = applyResize(start, 'w', 40, 0, MIN);
    expect(r.x).toBe(140);
    expect(r.w).toBe(420);
  });

  it('respects minimum width when shrinking past the floor', () => {
    const r = applyResize(start, 'e', -1000, 0, MIN);
    expect(r.w).toBe(MIN.w);
  });

  it('left-edge resize stops moving x once min width is hit', () => {
    const r = applyResize(start, 'w', 1000, 0, MIN);
    expect(r.w).toBe(MIN.w);
    // x cannot push past the right edge minus min width
    expect(r.x).toBe(start.x + (start.w - MIN.w));
  });
});
