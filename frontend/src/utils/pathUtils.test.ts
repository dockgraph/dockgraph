import { describe, it, expect } from 'vitest';
import {
  parsePolyline,
  polylineLength,
  polylineEndpoints,
  polylineBBox,
  polylinePointAt,
} from './pathUtils';

describe('parsePolyline', () => {
  it('returns empty array for empty string', () => {
    expect(parsePolyline('')).toEqual([]);
  });

  it('parses a simple M/L path', () => {
    const points = parsePolyline('M 0 0 L 10 20 L 30 40');
    expect(points).toEqual([
      { x: 0, y: 0 },
      { x: 10, y: 20 },
      { x: 30, y: 40 },
    ]);
  });

  it('handles negative coordinates', () => {
    const points = parsePolyline('M -5 -10 L 15 20');
    expect(points).toEqual([
      { x: -5, y: -10 },
      { x: 15, y: 20 },
    ]);
  });

  it('handles decimal values', () => {
    const points = parsePolyline('M 1.5 2.5 L 3.5 4.5');
    expect(points).toEqual([
      { x: 1.5, y: 2.5 },
      { x: 3.5, y: 4.5 },
    ]);
  });

  it('returns empty for single coordinate', () => {
    expect(parsePolyline('M 5')).toEqual([]);
  });
});

describe('polylineLength', () => {
  it('returns 0 for a single point', () => {
    expect(polylineLength([{ x: 0, y: 0 }])).toBe(0);
  });

  it('computes length of a horizontal segment', () => {
    const len = polylineLength([{ x: 0, y: 0 }, { x: 10, y: 0 }]);
    expect(len).toBe(10);
  });

  it('computes length of a multi-segment path', () => {
    const len = polylineLength([
      { x: 0, y: 0 },
      { x: 3, y: 4 },
      { x: 3, y: 14 },
    ]);
    expect(len).toBe(15); // 5 + 10
  });

  it('returns 0 for empty array', () => {
    expect(polylineLength([])).toBe(0);
  });
});

describe('polylineEndpoints', () => {
  it('returns null for fewer than 2 points', () => {
    expect(polylineEndpoints([{ x: 5, y: 5 }])).toBeNull();
    expect(polylineEndpoints([])).toBeNull();
  });

  it('returns first and last points', () => {
    const ep = polylineEndpoints([
      { x: 1, y: 2 },
      { x: 3, y: 4 },
      { x: 5, y: 6 },
    ]);
    expect(ep).toEqual({ sx: 1, sy: 2, ex: 5, ey: 6 });
  });
});

describe('polylineBBox', () => {
  it('returns null for empty array', () => {
    expect(polylineBBox([])).toBeNull();
  });

  it('computes bounding box', () => {
    const bbox = polylineBBox([
      { x: 5, y: 10 },
      { x: -3, y: 20 },
      { x: 15, y: 0 },
    ]);
    expect(bbox).toEqual({ minX: -3, minY: 0, maxX: 15, maxY: 20 });
  });

  it('handles single point', () => {
    const bbox = polylineBBox([{ x: 7, y: 3 }]);
    expect(bbox).toEqual({ minX: 7, minY: 3, maxX: 7, maxY: 3 });
  });
});

describe('polylinePointAt', () => {
  const points = [{ x: 0, y: 0 }, { x: 10, y: 0 }];
  const totalLen = 10;

  it('returns first point at t=0', () => {
    expect(polylinePointAt(points, 0, totalLen)).toEqual({ x: 0, y: 0 });
  });

  it('returns last point at t=1', () => {
    expect(polylinePointAt(points, 1, totalLen)).toEqual({ x: 10, y: 0 });
  });

  it('returns midpoint at t=0.5', () => {
    expect(polylinePointAt(points, 0.5, totalLen)).toEqual({ x: 5, y: 0 });
  });

  it('clamps t below 0', () => {
    expect(polylinePointAt(points, -1, totalLen)).toEqual({ x: 0, y: 0 });
  });

  it('clamps t above 1', () => {
    expect(polylinePointAt(points, 2, totalLen)).toEqual({ x: 10, y: 0 });
  });

  it('interpolates on multi-segment path', () => {
    const multiPoints = [
      { x: 0, y: 0 },
      { x: 10, y: 0 },
      { x: 10, y: 10 },
    ];
    const len = 20;
    const pt = polylinePointAt(multiPoints, 0.75, len);
    expect(pt.x).toBe(10);
    expect(pt.y).toBe(5);
  });
});
