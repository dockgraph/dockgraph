import { memo } from 'react';
import {
  BaseEdge,
  EdgeLabelRenderer,
  Position,
  getBezierPath,
  useInternalNode,
  type EdgeProps,
  type InternalNode,
} from '@xyflow/react';
import { STATUS_COLORS, networkColor } from '../utils/colors';
import { SWARM_CONTROL_PORT, type SwarmLinkData } from '../utils/swarmLinks';
import { ARROW_LENGTH, ARROW_WIDTH, DOT_OPACITY, DOT_SPEED, MIN_ANIMATION_DURATION } from '../utils/constants';
import { useTheme } from '../theme';

// Floating-edge geometry, adapted from the React Flow "Simple Floating Edges"
// example (https://reactflow.dev/examples/edges/simple-floating-edges): the
// link leaves each node where the line between both centres crosses its
// border, so it follows the nodes wherever the role groups are dragged.

interface Rect { x: number; y: number; w: number; h: number }

function rectOf(node: InternalNode): Rect {
  const { x, y } = node.internals.positionAbsolute;
  return {
    x,
    y,
    w: node.measured.width ?? node.width ?? 0,
    h: node.measured.height ?? node.height ?? 0,
  };
}

/** Point where the segment from `a`'s centre to `b`'s centre leaves `a`. */
function intersection(a: Rect, b: Rect): { x: number; y: number } {
  const w = a.w / 2;
  const h = a.h / 2;
  const cx = a.x + w;
  const cy = a.y + h;
  if (w === 0 || h === 0) return { x: cx, y: cy };
  const dx = b.x + b.w / 2 - cx;
  const dy = b.y + b.h / 2 - cy;
  const xx1 = dx / (2 * w) - dy / (2 * h);
  const yy1 = dx / (2 * w) + dy / (2 * h);
  const k = 1 / (Math.abs(xx1) + Math.abs(yy1) || 1);
  const xx3 = k * xx1;
  const yy3 = k * yy1;
  return { x: w * (xx3 + yy3) + cx, y: h * (-xx3 + yy3) + cy };
}

/** Side of `r` the border point `p` lies on, for the bezier's tangent. */
function sideOf(r: Rect, p: { x: number; y: number }): Position {
  const px = Math.round(p.x);
  const py = Math.round(p.y);
  if (px <= Math.round(r.x) + 1) return Position.Left;
  if (px >= Math.round(r.x + r.w) - 1) return Position.Right;
  if (py <= Math.round(r.y) + 1) return Position.Top;
  return Position.Bottom;
}

/**
 * Link of the per-node view (see utils/swarmLinks.ts): one manager→worker
 * control-plane spoke, or an overlay link between two service cards on
 * different nodes that share a network.
 */
export const SwarmLinkEdge = memo(function SwarmLinkEdge({ id, source, target, data, style }: EdgeProps) {
  const { theme } = useTheme();
  const sourceNode = useInternalNode(source);
  const targetNode = useInternalNode(target);
  const link = data as unknown as SwarmLinkData | undefined;
  if (!sourceNode || !targetNode || !link) return null;

  const s = rectOf(sourceNode);
  const t = rectOf(targetNode);
  const sp = intersection(s, t);
  const tp = intersection(t, s);
  const [path, labelX, labelY] = getBezierPath({
    sourceX: sp.x,
    sourceY: sp.y,
    sourcePosition: sideOf(s, sp),
    targetX: tp.x,
    targetY: tp.y,
    targetPosition: sideOf(t, tp),
  });
  const opacity = (style?.opacity as number | undefined) ?? 1;

  if (link.kind === 'overlay') {
    const color = networkColor(link.networks[0] ?? '', link.stack);
    return (
      <g data-testid="swarm-overlay-link">
        <title>{`overlay: ${link.networks.join(', ')}`}</title>
        <BaseEdge id={id} path={path} style={{ stroke: color, strokeWidth: 1.5, opacity }} />
      </g>
    );
  }

  // Healthy spokes get the "signal" accent plus a traveling arrow (manager →
  // worker) instead of React Flow's generic dash animation, so the control
  // plane reads as its own live flow rather than a dependency edge. A down
  // worker stays a static dashed red line, labelled on its own spoke so the
  // specific worker stands out instead of only an aggregate count.
  const color = link.healthy ? theme.edgeSignal : STATUS_COLORS.exited;
  const dist = Math.hypot(tp.x - sp.x, tp.y - sp.y);
  const duration = Math.max(MIN_ANIMATION_DURATION, dist / DOT_SPEED);

  return (
    <>
      <BaseEdge
        id={id}
        path={path}
        style={{
          stroke: color,
          strokeWidth: 1.5,
          strokeDasharray: link.healthy ? undefined : '6 4',
          opacity,
        }}
      />
      {link.healthy && (
        <path
          // Triangle tip points along +x; animateMotion's rotate="auto" then
          // rotates it to the spoke's tangent (manager → worker direction).
          d={`M ${ARROW_LENGTH / 2} 0 L ${-ARROW_LENGTH / 2} ${ARROW_WIDTH} L ${-ARROW_LENGTH / 2} ${-ARROW_WIDTH} Z`}
          fill={color}
          opacity={opacity * DOT_OPACITY}
        >
          <animateMotion dur={`${duration}s`} repeatCount="indefinite" path={path} rotate="auto" />
        </path>
      )}
      {!link.healthy && (
        <EdgeLabelRenderer>
          <div
            data-testid="swarm-control-label"
            className="nodrag nopan"
            style={{
              position: 'absolute',
              transform: `translate(-50%, -50%) translate(${labelX}px, ${labelY}px)`,
              padding: '2px 8px',
              background: theme.canvasBg,
              border: `1px solid ${color}`,
              borderRadius: 6,
              fontFamily: 'var(--dg-font-mono)',
              fontSize: 10,
              lineHeight: 1.5,
              color,
              whiteSpace: 'nowrap',
              opacity,
              pointerEvents: 'none',
            }}
          >
            {`control · ${SWARM_CONTROL_PORT} · down`}
          </div>
        </EdgeLabelRenderer>
      )}
    </>
  );
});
