import { memo, useMemo } from 'react';
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
  DOT_RADIUS,
  DOT_OPACITY,
  ENDPOINT_RADIUS,
  DASH_PATTERN_SVG,
  DEFAULT_EDGE_STROKE_WIDTH,
  DEFAULT_EDGE_STROKE,
  zoomSelector,
} from '../utils/constants';

export const ElkEdge = memo(function ElkEdge({ data, style }: EdgeProps) {
  const edgeData = data as ElkEdgeData | undefined;
  const path = edgeData?.path;
  const active = edgeData?.active !== false;
  const animated = edgeData?.edgeType === 'depends_on' && (edgeData?.animated ?? active);
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
        style={{ cursor: 'pointer' }}
      />
    );
  }

  return (
    <g opacity={opacity} style={{ cursor: 'pointer' }}>
      <path
        d={path}
        fill="none"
        stroke="transparent"
        strokeWidth={CANVAS_EDGE_HIT_WIDTH}
      />
      <path
        d={path}
        fill="none"
        stroke={stroke}
        strokeWidth={strokeWidth}
        strokeDasharray={strokeDasharray}
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
          <circle key={i} r={DOT_RADIUS} fill={stroke} opacity={DOT_OPACITY}>
            <animateMotion
              dur={`${dur}s`}
              repeatCount="indefinite"
              begin={`${offset * dur}s`}
              path={path}
            />
          </circle>
        );
      })}
    </g>
  );
});
