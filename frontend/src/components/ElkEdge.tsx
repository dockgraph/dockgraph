import type { EdgeProps } from '@xyflow/react';

export function ElkEdge({ data, style }: EdgeProps) {
  const edgeData = data as Record<string, string> | undefined;
  const path = edgeData?.path;
  if (!path) return null;

  const animated = edgeData?.edgeType === 'depends_on';

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
      {animated && [0, 0.33, 0.66].map((offset) => (
        <circle key={offset} r={1.8} fill={stroke} opacity={0.6}>
          <animateMotion
            dur="3s"
            repeatCount="indefinite"
            begin={`${offset * 3}s`}
            path={path}
          />
        </circle>
      ))}
    </g>
  );
}
