import type { EdgeProps } from '@xyflow/react';

// Renders ELK-computed orthogonal edge paths with connection dots at endpoints.
export function ElkEdge({ data, style }: EdgeProps) {
  const path = (data as Record<string, string>)?.path;
  if (!path) return null;

  const stroke = (style?.stroke as string) ?? '#475569';
  const strokeWidth = (style?.strokeWidth as number) ?? 1;
  const strokeDasharray = style?.strokeDasharray as string | undefined;
  const opacity = (style?.opacity as number) ?? 1;

  const coords = path.match(/[\d.]+/g)?.map(Number) ?? [];
  const startX = coords[0];
  const startY = coords[1];
  const endX = coords[coords.length - 2];
  const endY = coords[coords.length - 1];

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
    </g>
  );
}
