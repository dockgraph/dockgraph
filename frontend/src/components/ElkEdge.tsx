import { memo, useCallback, useMemo } from 'react';
import type { EdgeProps } from '@xyflow/react';
import { useStore } from '@xyflow/react';
import type { ElkEdgeData } from '../types';
import { parsePolyline, polylineLength, polylineEndpoints } from '../utils/pathUtils';
import {
  ANIMATION_NODE_LIMIT,
  CANVAS_EDGE_HIT_WIDTH,
  DOT_SPEED,
  MIN_ANIMATION_DURATION,
  DOT_SPACING,
  MIN_DOTS,
  MAX_DOTS,
  ARROW_LENGTH,
  ARROW_WIDTH,
  DOT_OPACITY,
  ENDPOINT_RADIUS,
  DASH_PATTERN_SVG,
  DEFAULT_EDGE_STROKE_WIDTH,
  DEFAULT_EDGE_STROKE,
  zoomSelector,
} from '../utils/constants';

export const ElkEdge = memo(function ElkEdge({ id, data, style }: EdgeProps) {
  const edgeData = data as ElkEdgeData | undefined;

  const handleClick = useCallback((e: React.MouseEvent) => {
    e.stopPropagation();
    document.dispatchEvent(new CustomEvent('dg:edge-click', { detail: id }));
  }, [id]);
  const path = edgeData?.path;
  const active = edgeData?.active !== false;
  // Service dependencies (db) and storage mounts carry the "live" signal —
  // render them in the accent/orange strokes with a traveling arrow.
  const isLiveEdgeType = edgeData?.edgeType === 'depends_on' || edgeData?.edgeType === 'volume_mount';
  const animated = isLiveEdgeType && (edgeData?.animated ?? active);
  const isLowZoom = useStore(zoomSelector);
  const isSimplified = (edgeData?.nodeCount ?? 0) > ANIMATION_NODE_LIMIT;

  const points = useMemo(() => (path ? parsePolyline(path) : []), [path]);
  const ep = useMemo(() => polylineEndpoints(points), [points]);

  const { dur, dotCount } = useMemo(() => {
    if (!animated || !path) return { dur: 0, dotCount: 0 };
    const length = polylineLength(points);
    return {
      dur: Math.max(MIN_ANIMATION_DURATION, length / DOT_SPEED),
      dotCount: Math.min(MAX_DOTS, Math.max(MIN_DOTS, Math.round(length / DOT_SPACING))),
    };
  }, [animated, path, points]);

  if (!path) return null;

  // At low zoom on large graphs, edges are sub-pixel — skip rendering entirely.
  if (isLowZoom && isSimplified) return null;

  const stroke = (style?.stroke as string) ?? DEFAULT_EDGE_STROKE;
  const strokeWidth = (style?.strokeWidth as number) ?? DEFAULT_EDGE_STROKE_WIDTH;
  const strokeDasharray = !active ? DASH_PATTERN_SVG : (style?.strokeDasharray as string | undefined);
  const opacity = (style?.opacity as number) ?? 1;

  if (isSimplified) {
    return (
      <path
        d={path}
        fill="none"
        stroke={stroke}
        strokeWidth={strokeWidth}
        strokeDasharray={strokeDasharray}
        opacity={opacity}
        onClick={handleClick}
        style={{ cursor: 'pointer', pointerEvents: 'stroke' }}
      />
    );
  }

  return (
    <g opacity={opacity}>
      {/* Wide transparent hit area for clicks — pointer-events: stroke responds even for transparent fills */}
      <path
        d={path}
        fill="none"
        stroke="transparent"
        strokeWidth={CANVAS_EDGE_HIT_WIDTH}
        onClick={handleClick}
        style={{ cursor: 'pointer', pointerEvents: 'stroke' }}
      />
      <path
        d={path}
        fill="none"
        stroke={stroke}
        strokeWidth={strokeWidth}
        strokeDasharray={strokeDasharray}
        style={{ pointerEvents: 'none' }}
      />
      {ep && (
        <circle cx={ep.sx} cy={ep.sy} r={ENDPOINT_RADIUS} fill={stroke} />
      )}
      {ep && (
        <circle cx={ep.ex} cy={ep.ey} r={ENDPOINT_RADIUS} fill={stroke} />
      )}
      {animated && Array.from({ length: dotCount }, (_, i) => {
        const offset = i / dotCount;
        return (
          <path
            key={i}
            // Triangle tip points along +x; animateMotion's rotate="auto"
            // then rotates it to the path's tangent at each point.
            d={`M ${ARROW_LENGTH / 2} 0 L ${-ARROW_LENGTH / 2} ${ARROW_WIDTH} L ${-ARROW_LENGTH / 2} ${-ARROW_WIDTH} Z`}
            fill={stroke}
            opacity={DOT_OPACITY}
          >
            <animateMotion
              dur={`${dur}s`}
              repeatCount="indefinite"
              begin={`${offset * dur}s`}
              path={path}
              rotate="auto"
            />
          </path>
        );
      })}
    </g>
  );
});
