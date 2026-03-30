import { describe, it, expect } from 'vitest';
import { resolveEdgeStyle, viewBounds, isVisible } from './canvasEdgeUtils';
import { DEFAULT_EDGE_STROKE, DEFAULT_EDGE_STROKE_WIDTH } from '../utils/constants';

// --- resolveEdgeStyle ---

describe('resolveEdgeStyle', () => {
  it('returns defaults for undefined style', () => {
    const result = resolveEdgeStyle(undefined);

    expect(result.stroke).toBe(DEFAULT_EDGE_STROKE);
    expect(result.lineWidth).toBe(DEFAULT_EDGE_STROKE_WIDTH);
    expect(result.opacity).toBe(1);
  });

  it('returns defaults for empty style object', () => {
    const result = resolveEdgeStyle({});

    expect(result).toEqual({
      stroke: DEFAULT_EDGE_STROKE,
      lineWidth: DEFAULT_EDGE_STROKE_WIDTH,
      opacity: 1,
    });
  });

  it('extracts all provided properties', () => {
    const result = resolveEdgeStyle({
      stroke: '#ff0000',
      strokeWidth: 3,
      opacity: 0.5,
    });

    expect(result).toEqual({ stroke: '#ff0000', lineWidth: 3, opacity: 0.5 });
  });

  it('falls back on individual missing properties', () => {
    const result = resolveEdgeStyle({ stroke: '#00ff00' });

    expect(result.stroke).toBe('#00ff00');
    expect(result.lineWidth).toBe(DEFAULT_EDGE_STROKE_WIDTH);
    expect(result.opacity).toBe(1);
  });
});

// --- viewBounds ---

describe('viewBounds', () => {
  // CULL_PAD = 20 (internal constant)

  it('computes bounds at identity transform (zoom=1, no translation)', () => {
    const vb = viewBounds({ tx: 0, ty: 0, zoom: 1 }, 800, 600);

    expect(vb.left).toBe(-20);
    expect(vb.top).toBe(-20);
    expect(vb.right).toBe(820);
    expect(vb.bottom).toBe(620);
  });

  it('zooming in shrinks the visible area in flow coordinates', () => {
    const vb = viewBounds({ tx: 0, ty: 0, zoom: 2 }, 800, 600);

    expect(vb.right).toBe(420);  // 800/2 + 20
    expect(vb.bottom).toBe(320); // 600/2 + 20
  });

  it('zooming out expands the visible area', () => {
    const vb = viewBounds({ tx: 0, ty: 0, zoom: 0.5 }, 800, 600);

    expect(vb.right).toBe(1620); // 800/0.5 + 20
    expect(vb.bottom).toBe(1220);
  });

  it('positive translation shifts the viewport origin negative', () => {
    const vb = viewBounds({ tx: 100, ty: 200, zoom: 1 }, 800, 600);

    expect(vb.left).toBe(-120);  // -100/1 - 20
    expect(vb.top).toBe(-220);
    expect(vb.right).toBe(720);  // (800-100)/1 + 20
    expect(vb.bottom).toBe(420);
  });

  it('handles combined zoom + translation', () => {
    const vb = viewBounds({ tx: 50, ty: 100, zoom: 0.5 }, 400, 300);

    expect(vb.left).toBe(-120);  // -50/0.5 - 20
    expect(vb.top).toBe(-220);
    expect(vb.right).toBe(720);  // (400-50)/0.5 + 20
    expect(vb.bottom).toBe(420);
  });
});

// --- isVisible ---

describe('isVisible', () => {
  const vb = { left: 0, top: 0, right: 100, bottom: 100 };

  it('detects fully contained bbox as visible', () => {
    expect(isVisible({ minX: 10, minY: 10, maxX: 90, maxY: 90 }, vb)).toBe(true);
  });

  it('detects partially overlapping bbox as visible', () => {
    expect(isVisible({ minX: -50, minY: -50, maxX: 10, maxY: 10 }, vb)).toBe(true);
  });

  it('detects bbox enclosing the viewport as visible', () => {
    expect(isVisible({ minX: -100, minY: -100, maxX: 200, maxY: 200 }, vb)).toBe(true);
  });

  it('rejects bbox entirely to the left', () => {
    expect(isVisible({ minX: -100, minY: 10, maxX: -1, maxY: 90 }, vb)).toBe(false);
  });

  it('rejects bbox entirely above', () => {
    expect(isVisible({ minX: 10, minY: -100, maxX: 90, maxY: -1 }, vb)).toBe(false);
  });

  it('rejects bbox entirely to the right', () => {
    expect(isVisible({ minX: 101, minY: 10, maxX: 200, maxY: 90 }, vb)).toBe(false);
  });

  it('rejects bbox entirely below', () => {
    expect(isVisible({ minX: 10, minY: 101, maxX: 90, maxY: 200 }, vb)).toBe(false);
  });

  it('accepts bbox touching at exactly the boundary', () => {
    expect(isVisible({ minX: 100, minY: 0, maxX: 200, maxY: 100 }, vb)).toBe(true);
    expect(isVisible({ minX: -50, minY: 100, maxX: 50, maxY: 200 }, vb)).toBe(true);
  });
});
