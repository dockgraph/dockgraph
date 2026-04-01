import { memo, useCallback } from 'react';
import type { NodeProps } from '@xyflow/react';
import { useStore } from '@xyflow/react';
import { NodeHandles } from './NodeHandles';
import { useTheme } from '../theme';
import type { VolumeNodeData } from '../types';
import { INACTIVE_OPACITY, zoomSelector } from '../utils/constants';

export const VolumeNode = memo(function VolumeNode({ data }: NodeProps) {
  const { dgNode, nodeWidth, onInfoClick } = data as unknown as VolumeNodeData;
  const w = nodeWidth ?? 200;
  const { theme } = useTheme();
  const isLowZoom = useStore(zoomSelector);
  const isGhost = dgNode.status === 'not_running';
  const opacity = isGhost ? INACTIVE_OPACITY : 1;

  const handleInfo = useCallback((e: React.MouseEvent) => {
    e.stopPropagation();
    onInfoClick?.(dgNode.id);
  }, [onInfoClick, dgNode.id]);

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
      <div style={{ overflow: 'hidden', flex: 1, minWidth: 0 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
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
          {onInfoClick && (
            <button
              onClick={handleInfo}
              aria-label={`Inspect ${dgNode.name}`}
              title="Inspect volume"
              style={{
                width: 14,
                height: 14,
                borderRadius: 2,
                border: 'none',
                background: 'transparent',
                cursor: 'pointer',
                padding: 0,
                display: 'flex',
                flexDirection: 'column',
                alignItems: 'flex-end',
                justifyContent: 'center',
                gap: 2,
                flexShrink: 0,
              }}
            >
              <span style={{ width: 10, height: 2, background: theme.nodeSubtext, borderRadius: 1, display: 'block' }} />
              <span style={{ width: 10, height: 2, background: theme.nodeSubtext, borderRadius: 1, display: 'block' }} />
              <span style={{ width: 7, height: 2, background: theme.nodeSubtext, borderRadius: 1, display: 'block' }} />
            </button>
          )}
        </div>
        {dgNode.driver && (
          <div style={{ fontSize: 9, color: theme.nodeSubtext }}>{dgNode.driver}</div>
        )}
      </div>

    </div>
  );
});
