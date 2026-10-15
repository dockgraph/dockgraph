import { memo } from 'react';
import type { NodeProps } from '@xyflow/react';
import { SWARM_CONTROL_PORT } from '../utils/swarmLinks';
import { STATUS_DOT_SIZE } from '../utils/constants';
import { chip } from '../utils/chipStyle';
import { useTheme } from '../theme';
import type { ControlSummaryData } from '../types';

/**
 * Free-standing badge of the per-node view, centered in the gap between the
 * Managers and Workers groups (see layout/nodeLayout.ts). The control link
 * fans out to one spoke per worker (see SwarmLinkEdge), so this is the only
 * place carrying the aggregate reading: protocol, port, and ready count, in
 * the same chip look as the node boxes' own badges.
 */
export const ControlSummary = memo(function ControlSummary({ data }: NodeProps) {
  const { ready, total, healthy } = data as unknown as ControlSummaryData;
  const { theme } = useTheme();
  // Blue / red rather than the spokes' teal, so the badge never blends into
  // the animated links running behind it.
  const color = healthy ? theme.info : theme.danger;
  const down = total - ready;
  const label = healthy
    ? `control / raft tcp ${SWARM_CONTROL_PORT} / ${ready}/${total} ok`
    : `control / raft tcp ${SWARM_CONTROL_PORT} / ${down}/${total} down`;

  return (
    <div
      data-testid="control-summary"
      title={`Raft control plane on TCP ${SWARM_CONTROL_PORT}`}
      // One soft-tinted pill, status dot included, centered on the (x, y)
      // anchor nodeLayout.ts computed via translate(-50%, -50%) — a single
      // shape, so its visual center is the anchor. The mostly opaque canvas
      // underlay keeps the spokes behind it from showing through the tint.
      style={{
        ...chip,
        transform: 'translate(-50%, -50%)',
        display: 'inline-flex',
        alignItems: 'center',
        gap: 6,
        borderRadius: 9999,
        padding: '2px 10px',
        fontSize: 10,
        color,
        background: `linear-gradient(${color}1f, ${color}1f), ${theme.canvasBg}d9`,
        border: `1px solid ${color}73`,
        pointerEvents: 'none',
      }}
    >
      <span
        aria-hidden="true"
        style={{
          width: STATUS_DOT_SIZE,
          height: STATUS_DOT_SIZE,
          borderRadius: '50%',
          background: color,
          flex: '0 0 auto',
        }}
      />
      {label}
    </div>
  );
});
