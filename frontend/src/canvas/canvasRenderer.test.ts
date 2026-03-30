// @vitest-environment jsdom
import { describe, it, expect, vi } from 'vitest';
import { drawEdges, drawAnimatedDots, syncCanvasSize } from './canvasRenderer';
import { DASH_PATTERN } from '../utils/constants';
import type { CanvasEdge, AnimatedEdge, ViewBounds } from './canvasEdgeTypes';

function mockCtx() {
  return {
    globalAlpha: 1,
    strokeStyle: '',
    lineWidth: 1,
    fillStyle: '',
    save: vi.fn(),
    restore: vi.fn(),
    setLineDash: vi.fn(),
    stroke: vi.fn(),
    fill: vi.fn(),
    beginPath: vi.fn(),
    arc: vi.fn(),
    clearRect: vi.fn(),
    scale: vi.fn(),
    translate: vi.fn(),
  } as unknown as CanvasRenderingContext2D;
}

const VISIBLE_BBOX = { minX: 10, minY: 10, maxX: 90, maxY: 90 };
const OFFSCREEN_BBOX = { minX: 500, minY: 500, maxX: 600, maxY: 600 };
const VB: ViewBounds = { left: 0, top: 0, right: 100, bottom: 100 };

function makeCanvasEdge(overrides: Partial<CanvasEdge> = {}): CanvasEdge {
  return {
    id: 'e1',
    path: {} as Path2D,
    stroke: '#475569',
    lineWidth: 1,
    opacity: 1,
    dashed: false,
    startX: 10,
    startY: 10,
    endX: 90,
    endY: 90,
    bbox: VISIBLE_BBOX,
    ...overrides,
  };
}

// --- drawEdges ---

describe('drawEdges', () => {
  it('strokes visible edges and draws endpoint circles', () => {
    const ctx = mockCtx();
    drawEdges(ctx, [makeCanvasEdge()], VB);

    expect(ctx.stroke).toHaveBeenCalledTimes(1);
    expect(ctx.arc).toHaveBeenCalledTimes(2);
    expect(ctx.fill).toHaveBeenCalledTimes(2);
  });

  it('skips edges outside the viewport', () => {
    const ctx = mockCtx();
    drawEdges(ctx, [makeCanvasEdge({ bbox: OFFSCREEN_BBOX })], VB);

    expect(ctx.stroke).not.toHaveBeenCalled();
    expect(ctx.arc).not.toHaveBeenCalled();
  });

  it('does nothing for empty edge array', () => {
    const ctx = mockCtx();
    drawEdges(ctx, [], VB);

    expect(ctx.stroke).not.toHaveBeenCalled();
  });

  it('applies dash pattern for dashed edges and resets after', () => {
    const ctx = mockCtx();
    drawEdges(ctx, [makeCanvasEdge({ dashed: true })], VB);

    const calls = (ctx.setLineDash as ReturnType<typeof vi.fn>).mock.calls;
    expect(calls[0][0]).toEqual([...DASH_PATTERN]);
    expect(calls[1][0]).toEqual([]);
  });

  it('uses solid lines for non-dashed edges', () => {
    const ctx = mockCtx();
    drawEdges(ctx, [makeCanvasEdge({ dashed: false })], VB);

    const calls = (ctx.setLineDash as ReturnType<typeof vi.fn>).mock.calls;
    expect(calls[0][0]).toEqual([]);
  });

  it('sets the correct visual properties per edge', () => {
    const ctx = mockCtx();
    drawEdges(ctx, [makeCanvasEdge({ stroke: '#ff0000', opacity: 0.5, lineWidth: 3 })], VB);

    expect(ctx.strokeStyle).toBe('#ff0000');
    expect(ctx.globalAlpha).toBe(0.5);
    expect(ctx.lineWidth).toBe(3);
  });

  it('draws multiple visible edges independently', () => {
    const ctx = mockCtx();
    const edges = [
      makeCanvasEdge({ id: 'e1', stroke: '#ff0000' }),
      makeCanvasEdge({ id: 'e2', stroke: '#00ff00' }),
    ];
    drawEdges(ctx, edges, VB);

    expect(ctx.stroke).toHaveBeenCalledTimes(2);
    expect(ctx.arc).toHaveBeenCalledTimes(4); // 2 endpoints per edge
  });
});

// --- drawAnimatedDots ---

describe('drawAnimatedDots', () => {
  it('returns early for empty animated array', () => {
    const ctx = mockCtx();
    drawAnimatedDots(ctx, [], VB, 1000);

    expect(ctx.beginPath).not.toHaveBeenCalled();
  });

  it('draws the correct number of dots for visible edges', () => {
    const ctx = mockCtx();
    const anim: AnimatedEdge = {
      stroke: '#000',
      opacity: 1,
      points: [{ x: 0, y: 0 }, { x: 100, y: 0 }],
      totalLength: 100,
      duration: 2,
      dotCount: 3,
      bbox: VISIBLE_BBOX,
    };

    drawAnimatedDots(ctx, [anim], VB, 1000);

    expect(ctx.arc).toHaveBeenCalledTimes(3);
    expect(ctx.fill).toHaveBeenCalledTimes(3);
  });

  it('skips off-screen animated edges', () => {
    const ctx = mockCtx();
    const anim: AnimatedEdge = {
      stroke: '#000',
      opacity: 1,
      points: [{ x: 500, y: 500 }, { x: 600, y: 500 }],
      totalLength: 100,
      duration: 2,
      dotCount: 3,
      bbox: OFFSCREEN_BBOX,
    };

    drawAnimatedDots(ctx, [anim], VB, 1000);

    expect(ctx.arc).not.toHaveBeenCalled();
  });

  it('applies DOT_OPACITY to globalAlpha', () => {
    const ctx = mockCtx();
    const anim: AnimatedEdge = {
      stroke: '#000',
      opacity: 0.8,
      points: [{ x: 0, y: 0 }, { x: 50, y: 0 }],
      totalLength: 50,
      duration: 1,
      dotCount: 1,
      bbox: VISIBLE_BBOX,
    };

    drawAnimatedDots(ctx, [anim], VB, 0);

    // globalAlpha = opacity * DOT_OPACITY (0.8 * 0.6 = 0.48)
    expect(ctx.globalAlpha).toBeCloseTo(0.48, 1);
  });
});

// --- syncCanvasSize ---

describe('syncCanvasSize', () => {
  it('resizes canvas buffer accounting for device pixel ratio', () => {
    const origDpr = globalThis.window?.devicePixelRatio;
    Object.defineProperty(window, 'devicePixelRatio', { value: 2, configurable: true });

    const canvas = {
      width: 0,
      height: 0,
      style: { width: '', height: '' },
    } as unknown as HTMLCanvasElement;

    syncCanvasSize(canvas, 800, 600);

    expect(canvas.width).toBe(1600);
    expect(canvas.height).toBe(1200);
    expect(canvas.style.width).toBe('800px');
    expect(canvas.style.height).toBe('600px');

    Object.defineProperty(window, 'devicePixelRatio', { value: origDpr ?? 1, configurable: true });
  });

  it('skips resize when buffer already matches', () => {
    Object.defineProperty(window, 'devicePixelRatio', { value: 1, configurable: true });

    const canvas = {
      width: 800,
      height: 600,
      style: { width: '800px', height: '600px' },
    } as unknown as HTMLCanvasElement;

    syncCanvasSize(canvas, 800, 600);

    // Unchanged — no unnecessary buffer reallocations
    expect(canvas.width).toBe(800);
    expect(canvas.height).toBe(600);
  });
});
