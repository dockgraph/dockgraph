import { memo } from 'react';
import type { NodeProps } from '@xyflow/react';
import { useStore } from '@xyflow/react';
import { NodeHandles } from './NodeHandles';
import { useTheme } from '../theme';
import type { VolumeNodeData } from '../types';
import { INACTIVE_OPACITY, zoomSelector } from '../utils/constants';

export const VolumeNode = memo(function VolumeNode({ data }: NodeProps) {
  const { dgNode, nodeWidth } = data as unknown as VolumeNodeData;
  const w = nodeWidth ?? 200;
  const { theme } = useTheme();
  const isLowZoom = useStore(zoomSelector);
  const isGhost = dgNode.status === 'not_running';
  const opacity = isGhost ? INACTIVE_OPACITY : 1;

  if (isLowZoom) {
    return (
      <div
        style={{
          background: theme.nodeBg,
          borderLeft: `3px solid ${theme.nodeSubtext}`,
          borderRadius: 4,
          width: w,
          height: 40,
          boxSizing: 'border-box',
          opacity,
          overflow: 'hidden',
          padding: '5px 10px',
        }}
      >
        <NodeHandles />
      </div>
    );
  }

  return (
    <div
      style={{
        background: theme.nodeBg,
        border: `1px ${isGhost ? 'dashed' : 'solid'} ${isGhost ? theme.nodeGhostBorder : theme.nodeBorder}`,
        borderLeft: `3px solid ${theme.nodeSubtext}`,
        borderRadius: 4,
        padding: '5px 10px',
        display: 'flex',
        alignItems: 'center',
        gap: 6,
        width: w,
        height: 40,
        boxSizing: 'border-box',
        opacity,
      }}
    >
      <NodeHandles />

      <span style={{ fontSize: 14 }}>💾</span>
      <div style={{ overflow: 'hidden' }}>
        <div
          style={{
            fontSize: 11,
            fontWeight: 600,
            fontFamily: 'sans-serif',
            color: theme.nodeText,
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
          }}
          title={dgNode.name}
        >
          {dgNode.name}
        </div>
        {dgNode.driver && (
          <div style={{ fontSize: 9, color: theme.nodeSubtext }}>{dgNode.driver}</div>
        )}
      </div>

    </div>
  );
});
