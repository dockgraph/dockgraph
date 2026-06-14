import { useCallback, useEffect, useRef } from 'react';

export type ResizeEdge = 'n' | 's' | 'e' | 'w' | 'ne' | 'nw' | 'se' | 'sw';

export interface Rect {
  x: number;
  y: number;
  w: number;
  h: number;
}
interface MinSize {
  w: number;
  h: number;
}

/**
 * Pure geometry for an edge/corner resize. dx/dy are total pointer deltas from
 * the drag start. Edges containing 'w'/'n' move the origin; minimums are
 * enforced without letting the origin overshoot.
 */
export function applyResize(start: Rect, edge: ResizeEdge, dx: number, dy: number, min: MinSize): Rect {
  let { x, y, w, h } = start;

  if (edge.includes('e')) w = Math.max(min.w, start.w + dx);
  if (edge.includes('s')) h = Math.max(min.h, start.h + dy);
  if (edge.includes('w')) {
    const newW = Math.max(min.w, start.w - dx);
    x = start.x + (start.w - newW);
    w = newW;
  }
  if (edge.includes('n')) {
    const newH = Math.max(min.h, start.h - dy);
    y = start.y + (start.h - newH);
    h = newH;
  }
  return { x: Math.round(x), y: Math.round(y), w: Math.round(w), h: Math.round(h) };
}

/**
 * Suppresses text selection for the duration of a drag/resize gesture and
 * returns a restore function. Without this, moving the pointer over document
 * text mid-drag selects it even though the grab handle sets user-select: none.
 */
export function suppressSelection(): () => void {
  const body = document.body;
  const prevUserSelect = body.style.userSelect;
  const prevCursor = body.style.cursor;
  body.style.userSelect = 'none';
  return () => {
    body.style.userSelect = prevUserSelect;
    body.style.cursor = prevCursor;
  };
}

interface DragHandlers {
  onMove: (x: number, y: number) => void;
  onResize: (rect: Rect) => void;
  /** Called once when a drag/resize gesture ends. */
  onEnd?: () => void;
  /** Called continuously during a title-bar drag with the live pointer position. */
  onDrag?: (clientX: number, clientY: number) => void;
}

/**
 * Wires pointer events for moving (title bar) and resizing (handles) a window.
 * Returns `startMove` and `startResize` to attach to onPointerDown.
 */
export function useDragResize(getRect: () => Rect, min: MinSize, handlers: DragHandlers) {
  // Keep the latest handlers without re-subscribing the pointer listeners.
  const handlersRef = useRef(handlers);
  useEffect(() => {
    handlersRef.current = handlers;
  });

  const startMove = useCallback(
    (e: React.PointerEvent) => {
      e.preventDefault();
      const rect = getRect();
      const offsetX = e.clientX - rect.x;
      const offsetY = e.clientY - rect.y;
      const restore = suppressSelection();

      const onPointerMove = (ev: PointerEvent) => {
        const nx = ev.clientX - offsetX;
        const ny = ev.clientY - offsetY;
        handlersRef.current.onMove(nx, ny);
        handlersRef.current.onDrag?.(ev.clientX, ev.clientY);
      };
      const onPointerUp = () => {
        window.removeEventListener('pointermove', onPointerMove);
        window.removeEventListener('pointerup', onPointerUp);
        restore();
        handlersRef.current.onEnd?.();
      };
      window.addEventListener('pointermove', onPointerMove);
      window.addEventListener('pointerup', onPointerUp);
    },
    [getRect],
  );

  const startResize = useCallback(
    (edge: ResizeEdge) => (e: React.PointerEvent) => {
      e.preventDefault();
      e.stopPropagation();
      const start = getRect();
      const startX = e.clientX;
      const startY = e.clientY;
      const restore = suppressSelection();

      const onPointerMove = (ev: PointerEvent) => {
        const rect = applyResize(start, edge, ev.clientX - startX, ev.clientY - startY, min);
        handlersRef.current.onResize(rect);
      };
      const onPointerUp = () => {
        window.removeEventListener('pointermove', onPointerMove);
        window.removeEventListener('pointerup', onPointerUp);
        restore();
        handlersRef.current.onEnd?.();
      };
      window.addEventListener('pointermove', onPointerMove);
      window.addEventListener('pointerup', onPointerUp);
    },
    [getRect, min],
  );

  return { startMove, startResize };
}
