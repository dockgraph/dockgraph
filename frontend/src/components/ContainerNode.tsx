import type { NodeProps } from '@xyflow/react';
import { NodeHandles } from './NodeHandles';
import { STATUS_COLORS } from '../utils/colors';
import { useTheme } from '../theme';
import { CONTAINER_NODE_HEIGHT, STATUS_DOT_SIZE } from '../utils/constants';
import type { ContainerNodeData } from '../types';

const ellipsis: React.CSSProperties = {
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap',
};

export function ContainerNode({ data }: NodeProps) {
  const { dfNode, nodeWidth } = data as unknown as ContainerNodeData;
  const w = (nodeWidth ?? 200) - 4; // subtract border widths
  const { theme } = useTheme();
  const statusColor = STATUS_COLORS[dfNode.status ?? 'exited'] ?? STATUS_COLORS.exited;
  const isGhost = dfNode.status === 'not_running';

  return (
    <div
      style={{
        background: theme.nodeBg,
        borderTop: `1px ${isGhost ? 'dashed' : 'solid'} ${isGhost ? theme.nodeGhostBorder : theme.nodeBorder}`,
        borderRight: `1px ${isGhost ? 'dashed' : 'solid'} ${isGhost ? theme.nodeGhostBorder : theme.nodeBorder}`,
        borderBottom: `1px ${isGhost ? 'dashed' : 'solid'} ${isGhost ? theme.nodeGhostBorder : theme.nodeBorder}`,
        borderLeft: `3px solid ${statusColor}`,
        borderRadius: 4,
        padding: '6px 10px',
        width: w,
        height: CONTAINER_NODE_HEIGHT,
        boxSizing: 'border-box',
        overflow: 'hidden',
        opacity: isGhost ? 0.5 : 1,
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
          title={dfNode.name}
        >
          {dfNode.name}
        </span>
        <span
          style={{
            width: STATUS_DOT_SIZE,
            height: STATUS_DOT_SIZE,
            borderRadius: '50%',
            background: statusColor,
            display: 'inline-block',
            transition: 'background 0.3s',
          }}
        />
      </div>

      {dfNode.image && (
        <div
          style={{
            ...ellipsis,
            fontSize: 10,
            color: theme.nodeSubtext,
            marginTop: 2,
          }}
          title={dfNode.image}
        >
          {dfNode.image}
        </div>
      )}

      {dfNode.ports && dfNode.ports.length > 0 && (
        <div style={{ marginTop: 4, display: 'flex', gap: 4, flexWrap: 'wrap' }}>
          {dfNode.ports.map((p) => (
            <span
              key={`${p.host}-${p.container}`}
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
        </div>
      )}

    </div>
  );
}
