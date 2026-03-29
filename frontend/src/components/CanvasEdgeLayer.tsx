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
  VIEWPORT_SETTLE_DELAY,
} from '../utils/constants';
import { parsePolyline, polylineLength, polylineEndpoints, polylineBBox } from '../utils/pathUtils';
import type { CanvasEdge, AnimatedEdge, CanvasEdgeLayerHandle } from '../canvas/canvasEdgeTypes';
import { getPath2D, resolveEdgeStyle } from '../canvas/canvasEdgeUtils';
import { renderFrame } from '../canvas/canvasRenderer';

export type { CanvasEdgeLayerHandle } from '../canvas/canvasEdgeTypes';

interface CanvasEdgeLayerProps {
  edges: RFEdge[];
}

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
          startX: ep.sx, startY: ep.sy,
          endX: ep.ex, endY: ep.ey,
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
    // - On settle: full quality redraw after VIEWPORT_SETTLE_DELAY
    // - When animations active: continuous rAF loop with viewport culling
    //
    // The settle watcher and animation loop share scope so the settle callback
    // can restart the rAF loop after a pan/zoom pause.
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

      function animTick(timestamp: number) {
        const data = edgeDataRef.current;
        if (data.animatedEdges.length === 0) {
          rafRef.current = 0;
          return;
        }
        // Skip rendering while the canvas is hidden during pan/zoom, and
        // stop the loop to avoid wasted rAF callbacks. The settle callback
        // restarts it after the viewport stops moving.
        if (canvas!.style.visibility === 'hidden') {
          rafRef.current = 0;
          return;
        }
        const [tx, ty, zoom] = storeApi.getState().transform;
        renderFrame(canvas!, ctx!, { tx, ty, zoom }, data.canvasEdges, data.animatedEdges, timestamp);
        rafRef.current = requestAnimationFrame(animTick);
      }

      function startAnimLoop() {
        if (rafRef.current) return;
        if (edgeDataRef.current.animatedEdges.length > 0) {
          rafRef.current = requestAnimationFrame(animTick);
        }
      }

      function scheduleSettle() {
        if (settleTimerRef.current != null) clearTimeout(settleTimerRef.current);
        settleTimerRef.current = setTimeout(() => {
          fullRedraw(performance.now());
          startAnimLoop();
        }, VIEWPORT_SETTLE_DELAY);
      }

      // Hide canvas during interaction, redraw on settle.
      const unsub = storeApi.subscribe((state) => {
        const [tx, ty, zoom] = state.transform;
        if (tx === drawnTx && ty === drawnTy && zoom === drawnZoom) return;
        canvas.style.visibility = 'hidden';
        scheduleSettle();
      });

      fullRedraw(performance.now());
      startAnimLoop();

      return () => {
        unsub();
        if (settleTimerRef.current != null) clearTimeout(settleTimerRef.current);
        if (rafRef.current) cancelAnimationFrame(rafRef.current);
        rafRef.current = 0;
      };
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
        aria-hidden="true"
        style={{ position: 'absolute', inset: 0, zIndex: 1, pointerEvents: 'none' }}
      />
    );
  },
);
