import type { NodeProps } from '@xyflow/react';
import { NodeHandles } from './NodeHandles';
import { useTheme } from '../theme';
import type { VolumeNodeData } from '../types';
import { INACTIVE_OPACITY } from '../utils/constants';

export function VolumeNode({ data }: NodeProps) {
  const { dfNode, nodeWidth } = data as unknown as VolumeNodeData;
  const w = (nodeWidth ?? 200) - 4;
  const { theme } = useTheme();
  const isGhost = dfNode.status === 'not_running';

  return (
    <div
      style={{
        background: theme.nodeBg,
        borderTop: `1px ${isGhost ? 'dashed' : 'solid'} ${isGhost ? theme.nodeGhostBorder : theme.nodeBorder}`,
        borderRight: `1px ${isGhost ? 'dashed' : 'solid'} ${isGhost ? theme.nodeGhostBorder : theme.nodeBorder}`,
        borderBottom: `1px ${isGhost ? 'dashed' : 'solid'} ${isGhost ? theme.nodeGhostBorder : theme.nodeBorder}`,
        borderLeft: '3px solid #94a3b8',
        borderRadius: 4,
        padding: '5px 10px',
        display: 'flex',
        alignItems: 'center',
        gap: 6,
        width: w,
        height: 40,
        boxSizing: 'border-box',
        opacity: isGhost ? INACTIVE_OPACITY : 1,
      }}
    >
      <NodeHandles />

      <span style={{ fontSize: 14 }}>💾</span>
      <div style={{ overflow: 'hidden' }}>
        <div
          style={{
            fontSize: 11,
            fontWeight: 600,
            color: theme.nodeText,
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
          }}
          title={dfNode.name}
        >
          {dfNode.name}
        </div>
        {dfNode.driver && (
          <div style={{ fontSize: 9, color: theme.nodeSubtext }}>{dfNode.driver}</div>
        )}
      </div>

    </div>
  );
}
