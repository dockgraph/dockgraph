import type { NodeProps } from '@xyflow/react';
import { NodeHandles } from './NodeHandles';
import { STATUS_COLORS } from '../utils/colors';
import { useTheme } from '../theme';
import type { DFNode } from '../types';

type ContainerNodeData = {
  dfNode: DFNode;
};

export function ContainerNode({ data }: NodeProps) {
  const { dfNode } = data as ContainerNodeData;
  const { theme } = useTheme();
  const statusColor = STATUS_COLORS[dfNode.status ?? 'stopped'] ?? STATUS_COLORS.stopped;
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
        width: 196,
        height: 65,
        boxSizing: 'border-box',
        overflow: 'hidden',
        opacity: isGhost ? 0.5 : 1,
      }}
    >
      <NodeHandles />

      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <span
          style={{
            fontSize: 12,
            fontWeight: 600,
            color: theme.nodeText,
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
            maxWidth: 170,
            display: 'inline-block',
          }}
          title={dfNode.name}
        >
          {dfNode.name}
        </span>
        <span
          style={{
            width: 6,
            height: 6,
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
            fontSize: 10,
            color: theme.nodeSubtext,
            marginTop: 2,
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
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
