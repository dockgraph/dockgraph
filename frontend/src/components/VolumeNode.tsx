import type { NodeProps } from '@xyflow/react';
import { NodeHandles } from './NodeHandles';
import { useTheme } from '../theme';
import type { DFNode } from '../types';

type VolumeNodeData = {
  dfNode: DFNode;
};

export function VolumeNode({ data }: NodeProps) {
  const { dfNode } = data as VolumeNodeData;
  const { theme } = useTheme();

  return (
    <div
      style={{
        background: theme.nodeBg,
        borderTop: `1px solid ${theme.nodeBorder}`,
        borderRight: `1px solid ${theme.nodeBorder}`,
        borderBottom: `1px solid ${theme.nodeBorder}`,
        borderLeft: '3px solid #f97316',
        borderRadius: 4,
        padding: '5px 10px',
        display: 'flex',
        alignItems: 'center',
        gap: 6,
        width: 196,
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
