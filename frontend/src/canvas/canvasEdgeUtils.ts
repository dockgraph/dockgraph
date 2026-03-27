import type { BBox } from '../utils/pathUtils';
import { DEFAULT_EDGE_STROKE, DEFAULT_EDGE_STROKE_WIDTH } from '../utils/constants';
import type { Viewport, ViewBounds } from './canvasEdgeTypes';

/** Viewport culling padding — edges within this margin of the viewport are still drawn. */
const CULL_PAD = 20;

/** Returns a cached Path2D for the given SVG path string, creating one if needed. */
export function getPath2D(svgPath: string, cache: Map<string, Path2D>): Path2D {
  let p = cache.get(svgPath);
  if (!p) {
    p = new Path2D(svgPath);
    cache.set(svgPath, p);
  }
  return p;
}

/** Extracts stroke, lineWidth, and opacity from an edge style with safe defaults. */
export function resolveEdgeStyle(style: Record<string, unknown> | undefined) {
  return {
    stroke: (style?.stroke as string) ?? DEFAULT_EDGE_STROKE,
    lineWidth: (style?.strokeWidth as number) ?? DEFAULT_EDGE_STROKE_WIDTH,
    opacity: (style?.opacity as number) ?? 1,
  };
}

/** Converts a viewport transform + container dimensions to flow-coordinate bounds. */
export function viewBounds(vp: Viewport, w: number, h: number): ViewBounds {
  return {
    left: -vp.tx / vp.zoom - CULL_PAD,
    top: -vp.ty / vp.zoom - CULL_PAD,
    right: (w - vp.tx) / vp.zoom + CULL_PAD,
    bottom: (h - vp.ty) / vp.zoom + CULL_PAD,
  };
}

/** Tests whether a bounding box intersects the visible viewport bounds. */
export function isVisible(bbox: BBox, vb: ViewBounds): boolean {
  return bbox.maxX >= vb.left && bbox.minX <= vb.right
      && bbox.maxY >= vb.top  && bbox.minY <= vb.bottom;
}
