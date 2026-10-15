import { DASH_PATTERN, DOT_OPACITY, ARROW_LENGTH, ARROW_WIDTH, ENDPOINT_RADIUS } from '../utils/constants';
import { polylinePointAt, polylineDirectionAt } from '../utils/pathUtils';
import type { CanvasEdge, AnimatedEdge, Viewport, ViewBounds } from './canvasEdgeTypes';
import { viewBounds, isVisible } from './canvasEdgeUtils';

const NO_DASH: number[] = [];

/** Strokes static edges with endpoint circles, skipping those outside the viewport. */
export function drawEdges(ctx: CanvasRenderingContext2D, edges: CanvasEdge[], vb: ViewBounds): void {
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

/** Draws one arrowhead marker at `pt`, oriented along unit vector `dir`. */
function drawArrowhead(ctx: CanvasRenderingContext2D, pt: { x: number; y: number }, dir: { x: number; y: number }): void {
  const nx = -dir.y;
  const ny = dir.x;
  const tipX = pt.x + (dir.x * ARROW_LENGTH) / 2;
  const tipY = pt.y + (dir.y * ARROW_LENGTH) / 2;
  const baseX = pt.x - (dir.x * ARROW_LENGTH) / 2;
  const baseY = pt.y - (dir.y * ARROW_LENGTH) / 2;

  ctx.beginPath();
  ctx.moveTo(tipX, tipY);
  ctx.lineTo(baseX + nx * ARROW_WIDTH, baseY + ny * ARROW_WIDTH);
  ctx.lineTo(baseX - nx * ARROW_WIDTH, baseY - ny * ARROW_WIDTH);
  ctx.closePath();
  ctx.fill();
}

/** Draws animated arrowheads traveling along depends_on and volume_mount edges, skipping off-screen edges. */
export function drawAnimatedDots(
  ctx: CanvasRenderingContext2D,
  animated: AnimatedEdge[],
  vb: ViewBounds,
  timestamp: number,
): void {
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
      const dir = polylineDirectionAt(anim.points, t, anim.totalLength);
      drawArrowhead(ctx, pt, dir);
    }
  }
}

/** Sizes the canvas buffer to match its container, accounting for device pixel ratio. */
export function syncCanvasSize(canvas: HTMLCanvasElement, w: number, h: number): void {
  const dpr = window.devicePixelRatio || 1;
  if (canvas.width !== w * dpr || canvas.height !== h * dpr) {
    canvas.width = w * dpr;
    canvas.height = h * dpr;
    canvas.style.width = `${w}px`;
    canvas.style.height = `${h}px`;
  }
}

/** Clears the canvas, applies the viewport transform, and draws all edges + animations. */
export function renderFrame(
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
