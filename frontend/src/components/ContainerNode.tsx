import { memo } from 'react';
import type { NodeProps } from '@xyflow/react';
import { useStore } from '@xyflow/react';
import { NodeHandles } from './NodeHandles';
import { STATUS_COLORS, STATUS_LABELS } from '../utils/colors';
import { useTheme } from '../theme';
import { CONTAINER_NODE_HEIGHT, STATUS_DOT_SIZE, INACTIVE_OPACITY, PAUSED_OPACITY, zoomSelector } from '../utils/constants';
import type { ContainerNodeData } from '../types';

const ellipsis: React.CSSProperties = {
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap',
};

export const ContainerNode = memo(function ContainerNode({ data }: NodeProps) {
  const { dgNode, nodeWidth } = data as unknown as ContainerNodeData;
  const w = (nodeWidth ?? 200) - 4;
  const { theme } = useTheme();
  const isLowZoom = useStore(zoomSelector);
  const statusColor = STATUS_COLORS[dgNode.status ?? 'exited'] ?? STATUS_COLORS.exited;
  const isGhost = dgNode.status === 'not_running';
  const isActive = dgNode.status === 'running' || dgNode.status === 'unhealthy';
  const isPaused = dgNode.status === 'paused';
  const opacity = isActive ? 1 : isPaused ? PAUSED_OPACITY : INACTIVE_OPACITY;

  // Simplified render at low zoom — just a colored block with the name.
  if (isLowZoom) {
    return (
      <div
        style={{
          background: theme.nodeBg,
          borderLeft: `3px solid ${statusColor}`,
          borderRadius: 4,
          width: w,
          height: CONTAINER_NODE_HEIGHT,
          boxSizing: 'border-box',
          opacity,
          overflow: 'hidden',
          padding: '6px 10px',
        }}
      >
        <NodeHandles />
        <span style={{ ...ellipsis, fontSize: 12, fontWeight: 600, color: theme.nodeText, display: 'block' }}>
          {dgNode.name}
        </span>
      </div>
    );
  }

  return (
    <div
      style={{
        background: theme.nodeBg,
        border: `1px ${isGhost ? 'dashed' : 'solid'} ${isGhost ? theme.nodeGhostBorder : theme.nodeBorder}`,
        borderLeft: `3px solid ${statusColor}`,
        borderRadius: 4,
        padding: '6px 10px',
        width: w,
        height: CONTAINER_NODE_HEIGHT,
        boxSizing: 'border-box',
        overflow: 'hidden',
        opacity,
      }}
    >
      <NodeHandles />

      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <span
          style={{
            ...ellipsis,
            fontSize: 12,
            fontWeight: 600,
            color: theme.nodeText,
            maxWidth: 'calc(100% - 16px)',
            display: 'inline-block',
          }}
          title={dgNode.name}
        >
          {dgNode.name}
        </span>
        <span
          role="img"
          aria-label={STATUS_LABELS[dgNode.status ?? 'exited'] ?? dgNode.status}
          title={STATUS_LABELS[dgNode.status ?? 'exited'] ?? dgNode.status}
          style={{
            width: STATUS_DOT_SIZE,
            height: STATUS_DOT_SIZE,
            borderRadius: '50%',
            background: isActive ? statusColor : 'transparent',
            border: isActive ? 'none' : `1.5px solid ${statusColor}`,
            display: 'inline-block',
            boxSizing: 'border-box',
            transition: 'background 0.3s, border-color 0.3s',
          }}
        />
      </div>

      {dgNode.image && (
        <div
          style={{
            ...ellipsis,
            fontSize: 10,
            color: theme.nodeSubtext,
            marginTop: 2,
          }}
          title={dgNode.image}
        >
          {dgNode.image}
        </div>
      )}

      {dgNode.ports && dgNode.ports.length > 0 && (
        <div style={{ marginTop: 4, display: 'flex', gap: 4, flexWrap: 'wrap' }}>
          {dgNode.ports.slice(0, 3).map((p, i) => (
            <span
              key={`${i}-${p.host}-${p.container}`}
              style={{
                fontSize: 9,
                color: theme.portText,
                background: theme.portBg,
                padding: '1px 4px',
                borderRadius: 3,
              }}
            >
              :{p.host} → {p.container}
            </span>
          ))}
          {dgNode.ports.length > 3 && (
            <span
              title={dgNode.ports.slice(3).map((p) => `:${p.host} → ${p.container}`).join(', ')}
              style={{ fontSize: 9, color: theme.nodeSubtext }}
            >
              +{dgNode.ports.length - 3}
            </span>
          )}
        </div>
      )}

    </div>
  );
});
