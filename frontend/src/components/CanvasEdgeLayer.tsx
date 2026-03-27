import { useEffect, useRef, useMemo, useImperativeHandle, forwardRef } from 'react';
import type { Edge as RFEdge } from '@xyflow/react';
import { useStoreApi } from '@xyflow/react';
import type { ElkEdgeData } from '../types';
import {
  CANVAS_EDGE_HIT_WIDTH,
  DOT_SPEED,
  MIN_ANIMATION_DURATION,
  DOT_SPACING,
  MIN_DOTS,
  MAX_DOTS,
  DOT_RADIUS,
  DOT_OPACITY,
  ENDPOINT_RADIUS,
  DASH_PATTERN,
  DEFAULT_EDGE_STROKE_WIDTH,
  DEFAULT_EDGE_STROKE,
  VIEWPORT_SETTLE_DELAY,
} from '../utils/constants';
import {
  type BBox,
  parsePolyline,
  polylineLength,
  polylineEndpoints,
  polylineBBox,
  polylinePointAt,
} from '../utils/pathUtils';

// ── Types ──────────────────────────────────────────────────────────

/** Parsed edge ready for canvas rendering. */
interface CanvasEdge {
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
interface AnimatedEdge {
  stroke: string;
  opacity: number;
  points: { x: number; y: number }[];
  totalLength: number;
  duration: number;
  dotCount: number;
  bbox: BBox;
}

export interface CanvasEdgeLayerHandle {
  hitTest: (screenX: number, screenY: number) => string | null;
}

interface CanvasEdgeLayerProps {
  edges: RFEdge[];
}

interface Viewport {
  tx: number;
  ty: number;
  zoom: number;
}

interface ViewBounds {
  left: number;
  top: number;
  right: number;
  bottom: number;
}

// ── Pure helpers ───────────────────────────────────────────────────

const NO_DASH: number[] = [];
const CULL_PAD = 20;

function getPath2D(svgPath: string, cache: Map<string, Path2D>): Path2D {
  let p = cache.get(svgPath);
  if (!p) {
    p = new Path2D(svgPath);
    cache.set(svgPath, p);
  }
  return p;
}

function resolveEdgeStyle(style: Record<string, unknown> | undefined) {
  return {
    stroke: (style?.stroke as string) ?? DEFAULT_EDGE_STROKE,
    lineWidth: (style?.strokeWidth as number) ?? DEFAULT_EDGE_STROKE_WIDTH,
    opacity: (style?.opacity as number) ?? 1,
  };
}

function viewBounds(vp: Viewport, w: number, h: number): ViewBounds {
  return {
    left: -vp.tx / vp.zoom - CULL_PAD,
    top: -vp.ty / vp.zoom - CULL_PAD,
    right: (w - vp.tx) / vp.zoom + CULL_PAD,
    bottom: (h - vp.ty) / vp.zoom + CULL_PAD,
  };
}

function isVisible(bbox: BBox, vb: ViewBounds): boolean {
  return bbox.maxX >= vb.left && bbox.minX <= vb.right
      && bbox.maxY >= vb.top  && bbox.minY <= vb.bottom;
}

// ── Drawing functions ──────────────────────────────────────────────

function drawEdges(ctx: CanvasRenderingContext2D, edges: CanvasEdge[], vb: ViewBounds): void {
  for (const edge of edges) {
    if (!isVisible(edge.bbox, vb)) continue;

    ctx.globalAlpha = edge.opacity;
    ctx.strokeStyle = edge.stroke;
    ctx.lineWidth = edge.lineWidth;
    ctx.setLineDash(edge.dashed ? DASH_PATTERN : NO_DASH);
    ctx.stroke(edge.path);

    ctx.setLineDash(NO_DASH);
    ctx.fillStyle = edge.stroke;
    ctx.beginPath();
    ctx.arc(edge.startX, edge.startY, ENDPOINT_RADIUS, 0, Math.PI * 2);
    ctx.fill();
    ctx.beginPath();
    ctx.arc(edge.endX, edge.endY, ENDPOINT_RADIUS, 0, Math.PI * 2);
    ctx.fill();
  }
}

function drawAnimatedDots(ctx: CanvasRenderingContext2D, animated: AnimatedEdge[], vb: ViewBounds, timestamp: number): void {
  if (animated.length === 0) return;
  const timeSec = timestamp / 1000;

  for (const anim of animated) {
    if (!isVisible(anim.bbox, vb)) continue;

    ctx.fillStyle = anim.stroke;
    ctx.globalAlpha = anim.opacity * DOT_OPACITY;

    for (let i = 0; i < anim.dotCount; i++) {
      const offset = i / anim.dotCount;
      const t = ((timeSec / anim.duration) + offset) % 1;
      const pt = polylinePointAt(anim.points, t, anim.totalLength);
      ctx.beginPath();
      ctx.arc(pt.x, pt.y, DOT_RADIUS, 0, Math.PI * 2);
      ctx.fill();
    }
  }
}

/** Sizes the canvas buffer to match its container, accounting for device pixel ratio. */
function syncCanvasSize(canvas: HTMLCanvasElement, w: number, h: number): void {
  const dpr = window.devicePixelRatio || 1;
  if (canvas.width !== w * dpr || canvas.height !== h * dpr) {
    canvas.width = w * dpr;
    canvas.height = h * dpr;
    canvas.style.width = `${w}px`;
    canvas.style.height = `${h}px`;
  }
}

function renderFrame(
  canvas: HTMLCanvasElement,
  ctx: CanvasRenderingContext2D,
  vp: Viewport,
  edges: CanvasEdge[],
  animated: AnimatedEdge[],
  timestamp: number,
): void {
  const parent = canvas.parentElement;
  if (!parent) return;

  const w = parent.clientWidth;
  const h = parent.clientHeight;
  syncCanvasSize(canvas, w, h);

  const dpr = window.devicePixelRatio || 1;
  ctx.clearRect(0, 0, canvas.width, canvas.height);
  ctx.save();
  ctx.scale(dpr, dpr);
  ctx.translate(vp.tx, vp.ty);
  ctx.scale(vp.zoom, vp.zoom);

  const vb = viewBounds(vp, w, h);
  drawEdges(ctx, edges, vb);
  drawAnimatedDots(ctx, animated, vb, timestamp);

  ctx.restore();
}

// ── Component ──────────────────────────────────────────────────────

/**
 * Renders bulk edges on a single <canvas> for high-performance rendering
 * at large graph sizes, replacing the per-edge SVG compositor bottleneck.
 *
 * The canvas has pointerEvents:none so it never blocks React Flow pan/zoom.
 * Edge click detection is exposed via the hitTest imperative handle.
 *
 * Viewport changes are read directly from the React Flow store inside
 * callbacks — zero React re-renders during interaction.
 */
export const CanvasEdgeLayer = forwardRef<CanvasEdgeLayerHandle, CanvasEdgeLayerProps>(
  function CanvasEdgeLayer({ edges }, ref) {
    const canvasRef = useRef<HTMLCanvasElement>(null);
    const rafRef = useRef<number>(0);
    const storeApi = useStoreApi();

    // Build static edge descriptors. Path2D cache is local so stale
    // entries are garbage-collected when edges change.
    const canvasEdges = useMemo(() => {
      const result: CanvasEdge[] = [];
      const cache = new Map<string, Path2D>();

      for (const edge of edges) {
        const data = edge.data as ElkEdgeData | undefined;
        const svgPath = data?.path;
        if (!svgPath) continue;

        const points = parsePolyline(svgPath);
        const ep = polylineEndpoints(points);
        const bbox = polylineBBox(points);
        if (!ep || !bbox) continue;

        const active = data?.active !== false;
        const style = resolveEdgeStyle(edge.style as Record<string, unknown>);

        result.push({
          id: edge.id,
          path: getPath2D(svgPath, cache),
          ...style,
          dashed: !active,
          startX: ep.sx,
          startY: ep.sy,
          endX: ep.ex,
          endY: ep.ey,
          bbox,
        });
      }

      return result;
    }, [edges]);

    // Build animated edge descriptors for depends_on edges with active endpoints.
    const animatedEdges = useMemo(() => {
      const result: AnimatedEdge[] = [];

      for (const edge of edges) {
        const data = edge.data as ElkEdgeData | undefined;
        if (!data?.path || !data.animated || data.edgeType !== 'depends_on') continue;

        const points = parsePolyline(data.path);
        if (points.length < 2) continue;

        const totalLength = polylineLength(points);
        const style = resolveEdgeStyle(edge.style as Record<string, unknown>);

        result.push({
          stroke: style.stroke,
          opacity: style.opacity,
          points,
          totalLength,
          duration: Math.max(MIN_ANIMATION_DURATION, totalLength / DOT_SPEED),
          dotCount: Math.min(MAX_DOTS, Math.max(MIN_DOTS, Math.round(totalLength / DOT_SPACING))),
          bbox: polylineBBox(points)!,
        });
      }

      return result;
    }, [edges]);

    // Ref holding the latest edge data for callbacks and rAF loop.
    const edgeDataRef = useRef({ canvasEdges, animatedEdges });
    useEffect(() => {
      edgeDataRef.current = { canvasEdges, animatedEdges };
      // Edge data changed (e.g. selection) — redraw immediately.
      const canvas = canvasRef.current;
      if (!canvas) return;
      const ctx = canvas.getContext('2d');
      if (!ctx) return;
      const [tx, ty, zoom] = storeApi.getState().transform;
      renderFrame(canvas, ctx, { tx, ty, zoom }, canvasEdges, animatedEdges, performance.now());
      canvas.style.visibility = 'visible';
    }, [canvasEdges, animatedEdges, storeApi]);

    // Expose hit-test to parent via imperative handle.
    useImperativeHandle(ref, () => ({
      hitTest(screenX: number, screenY: number): string | null {
        const canvas = canvasRef.current;
        if (!canvas) return null;
        const ctx = canvas.getContext('2d');
        if (!ctx) return null;

        const rect = canvas.getBoundingClientRect();
        const [tx, ty, zoom] = storeApi.getState().transform;
        const flowX = (screenX - rect.left - tx) / zoom;
        const flowY = (screenY - rect.top - ty) / zoom;

        ctx.lineWidth = CANVAS_EDGE_HIT_WIDTH / zoom;

        for (const edge of edgeDataRef.current.canvasEdges) {
          if (ctx.isPointInStroke(edge.path, flowX, flowY)) {
            return edge.id;
          }
        }

        return null;
      },
    }), [storeApi]);

    // Viewport-aware rendering strategy:
    // - During pan/zoom: hide canvas via CSS (zero draw cost, 60fps guaranteed)
    // - On settle (~80ms after last viewport change): full quality redraw
    // - When animations active: continuous rAF loop with viewport culling
    const settleTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

    useEffect(() => {
      const canvas = canvasRef.current;
      if (!canvas) return;
      const ctx = canvas.getContext('2d');
      if (!ctx) return;

      let drawnTx = NaN;
      let drawnTy = NaN;
      let drawnZoom = NaN;

      function fullRedraw(timestamp: number) {
        const [tx, ty, zoom] = storeApi.getState().transform;
        const data = edgeDataRef.current;
        renderFrame(canvas!, ctx!, { tx, ty, zoom }, data.canvasEdges, data.animatedEdges, timestamp);
        canvas!.style.visibility = 'visible';
        drawnTx = tx;
        drawnTy = ty;
        drawnZoom = zoom;
      }

      function scheduleSettle() {
        if (settleTimerRef.current != null) clearTimeout(settleTimerRef.current);
        settleTimerRef.current = setTimeout(() => fullRedraw(performance.now()), VIEWPORT_SETTLE_DELAY);
      }

      // Hide canvas during interaction, redraw on settle.
      const unsub = storeApi.subscribe((state) => {
        const [tx, ty, zoom] = state.transform;
        if (tx === drawnTx && ty === drawnTy && zoom === drawnZoom) return;
        canvas.style.visibility = 'hidden';
        scheduleSettle();
      });

      fullRedraw(performance.now());

      return () => {
        unsub();
        if (settleTimerRef.current != null) clearTimeout(settleTimerRef.current);
        if (rafRef.current) cancelAnimationFrame(rafRef.current);
      };
    }, [storeApi]);

    // Animation loop — only runs when animated edges exist.
    useEffect(() => {
      const canvas = canvasRef.current;
      if (!canvas) return;
      const ctx = canvas.getContext('2d');
      if (!ctx) return;
      if (animatedEdges.length === 0) return;

      function animTick(timestamp: number) {
        const [tx, ty, zoom] = storeApi.getState().transform;
        const data = edgeDataRef.current;
        if (data.animatedEdges.length === 0) return;
        if (canvas!.style.visibility !== 'hidden') {
          renderFrame(canvas!, ctx!, { tx, ty, zoom }, data.canvasEdges, data.animatedEdges, timestamp);
        }
        rafRef.current = requestAnimationFrame(animTick);
      }

      rafRef.current = requestAnimationFrame(animTick);
      return () => { if (rafRef.current) cancelAnimationFrame(rafRef.current); };
    }, [animatedEdges, storeApi]);

    // Redraw on container resize to maintain sharp rendering.
    useEffect(() => {
      const canvas = canvasRef.current;
      const parent = canvas?.parentElement;
      if (!parent || !canvas) return;
      const ctx = canvas.getContext('2d');
      if (!ctx) return;

      const observer = new ResizeObserver(() => {
        const [tx, ty, zoom] = storeApi.getState().transform;
        const data = edgeDataRef.current;
        renderFrame(canvas, ctx, { tx, ty, zoom }, data.canvasEdges, data.animatedEdges, performance.now());
      });

      observer.observe(parent);
      return () => observer.disconnect();
    }, [storeApi]);

    return (
      <canvas
        ref={canvasRef}
        style={{ position: 'absolute', inset: 0, zIndex: 1, pointerEvents: 'none' }}
      />
    );
  },
);
