import { useMemo } from 'react';
import type { EdgeProps } from '@xyflow/react';

const DOT_SPEED = 160;      // px per second — max travel speed
const MIN_DUR = 1.5;        // seconds — floor so short edges don't crawl
const DOT_SPACING = 150;    // px between dots — longer edges get more dots
const MIN_DOTS = 3;
const MAX_DOTS = 8;

function estimatePathLength(d: string): number {
  const coords = d.match(/-?[\d.]+/g)?.map(Number) ?? [];
  let length = 0;
  for (let i = 2; i < coords.length; i += 2) {
    const dx = coords[i] - coords[i - 2];
    const dy = coords[i + 1] - coords[i - 1];
    length += Math.sqrt(dx * dx + dy * dy);
  }
  return length;
}

export function ElkEdge({ data, style }: EdgeProps) {
  const edgeData = data as Record<string, string> | undefined;
  const path = edgeData?.path;
  if (!path) return null;

  const animated = edgeData?.edgeType === 'depends_on';

  const stroke = (style?.stroke as string) ?? '#475569';
  const strokeWidth = (style?.strokeWidth as number) ?? 1;
  const strokeDasharray = style?.strokeDasharray as string | undefined;
  const opacity = (style?.opacity as number) ?? 1;

  const coords = path.match(/-?[\d.]+/g)?.map(Number) ?? [];
  const startX = coords[0];
  const startY = coords[1];
  const endX = coords[coords.length - 2];
  const endY = coords[coords.length - 1];

  const { dur, dotCount } = useMemo(() => {
    if (!animated) return { dur: 0, dotCount: 0 };
    const length = estimatePathLength(path);
    return {
      dur: Math.max(MIN_DUR, length / DOT_SPEED),
      dotCount: Math.min(MAX_DOTS, Math.max(MIN_DOTS, Math.round(length / DOT_SPACING))),
    };
  }, [animated, path]);

  return (
    <g opacity={opacity}>
      <path
        d={path}
        fill="none"
        stroke={stroke}
        strokeWidth={strokeWidth}
        strokeDasharray={strokeDasharray}
      />
      {startX != null && (
        <circle cx={startX} cy={startY} r={2.5} fill={stroke} />
      )}
      {endX != null && (
        <circle cx={endX} cy={endY} r={2.5} fill={stroke} />
      )}
      {animated && Array.from({ length: dotCount }, (_, i) => {
        const offset = i / dotCount;
        return (
          <circle key={i} r={1.8} fill={stroke} opacity={0.6}>
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
}
