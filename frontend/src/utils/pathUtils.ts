/**
 * Utilities for working with SVG polyline path strings.
 * Used by both the canvas and SVG edge renderers.
 */

export interface Point {
  x: number;
  y: number;
}

export interface BBox {
  minX: number;
  minY: number;
  maxX: number;
  maxY: number;
}

/** Parses an SVG polyline path string ("M x y L x y ...") into coordinate pairs. */
export function parsePolyline(d: string): Point[] {
  const nums = d.match(/-?[\d.]+/g)?.map(Number);
  if (!nums || nums.length < 2) return [];
  if (nums.length % 2 !== 0) {
    console.warn('parsePolyline: odd coordinate count, last value dropped');
  }
  const points: Point[] = [];
  for (let i = 0; i < nums.length - 1; i += 2) {
    points.push({ x: nums[i], y: nums[i + 1] });
  }
  return points;
}

/** Computes total length of a polyline from its points. */
export function polylineLength(points: Point[]): number {
  let len = 0;
  for (let i = 1; i < points.length; i++) {
    const dx = points[i].x - points[i - 1].x;
    const dy = points[i].y - points[i - 1].y;
    len += Math.sqrt(dx * dx + dy * dy);
  }
  return len;
}

/** Returns the first and last points of a polyline as flat coordinates. */
export function polylineEndpoints(points: Point[]): { sx: number; sy: number; ex: number; ey: number } | null {
  if (points.length < 2) return null;
  return {
    sx: points[0].x,
    sy: points[0].y,
    ex: points[points.length - 1].x,
    ey: points[points.length - 1].y,
  };
}

/** Computes the axis-aligned bounding box of a set of points. */
export function polylineBBox(points: Point[]): BBox | null {
  if (points.length === 0) return null;
  let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
  for (const p of points) {
    if (p.x < minX) minX = p.x;
    if (p.x > maxX) maxX = p.x;
    if (p.y < minY) minY = p.y;
    if (p.y > maxY) maxY = p.y;
  }
  return { minX, minY, maxX, maxY };
}

/**
 * Interpolates a point at fraction t (0..1) along a polyline,
 * using a precomputed total length to avoid recalculating per call.
 */
export function polylinePointAt(points: Point[], t: number, totalLen: number): Point {
  if (points.length < 2 || t <= 0) return points[0];
  if (t >= 1) return points[points.length - 1];

  const targetDist = t * totalLen;
  let accumulated = 0;

  for (let i = 1; i < points.length; i++) {
    const dx = points[i].x - points[i - 1].x;
    const dy = points[i].y - points[i - 1].y;
    const segLen = Math.sqrt(dx * dx + dy * dy);

    if (accumulated + segLen >= targetDist) {
      const frac = segLen > 0 ? (targetDist - accumulated) / segLen : 0;
      return {
        x: points[i - 1].x + dx * frac,
        y: points[i - 1].y + dy * frac,
      };
    }
    accumulated += segLen;
  }

  return points[points.length - 1];
}
