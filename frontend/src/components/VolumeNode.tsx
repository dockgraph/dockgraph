import type { NodeProps } from '@xyflow/react';
import { NodeHandles } from './NodeHandles';
import { useTheme } from '../theme';
import type { VolumeNodeData } from '../types';

export function VolumeNode({ data }: NodeProps) {
  const { dfNode, nodeWidth } = data as unknown as VolumeNodeData;
  const w = (nodeWidth ?? 200) - 4;
  const { theme } = useTheme();

  return (
    <div
      style={{
        background: theme.nodeBg,
        borderTop: `1px solid ${theme.nodeBorder}`,
        borderRight: `1px solid ${theme.nodeBorder}`,
        borderBottom: `1px solid ${theme.nodeBorder}`,
        borderLeft: '3px solid #94a3b8',
        borderRadius: 4,
        padding: '5px 10px',
        display: 'flex',
        alignItems: 'center',
        gap: 6,
        width: w,
        height: 40,
        boxSizing: 'border-box',
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
