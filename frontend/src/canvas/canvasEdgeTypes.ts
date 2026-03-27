import type { BBox } from '../utils/pathUtils';

/** Parsed edge ready for canvas rendering. */
export interface CanvasEdge {
  id: string;
  path: Path2D;
  stroke: string;
  lineWidth: number;
  opacity: number;
  dashed: boolean;
  startX: number;
  startY: number;
  endX: number;
  endY: number;
  bbox: BBox;
}

/** Precomputed data for animated dots traveling along an edge path. */
export interface AnimatedEdge {
  stroke: string;
  opacity: number;
  points: { x: number; y: number }[];
  totalLength: number;
  duration: number;
  dotCount: number;
  bbox: BBox;
}

/** Imperative API exposed by CanvasEdgeLayer via ref. */
export interface CanvasEdgeLayerHandle {
  hitTest: (screenX: number, screenY: number) => string | null;
}

export interface Viewport {
  tx: number;
  ty: number;
  zoom: number;
}

export interface ViewBounds {
  left: number;
  top: number;
  right: number;
  bottom: number;
}
