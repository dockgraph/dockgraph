import { Handle, Position, type NodeProps } from '@xyflow/react';
import { STATUS_COLORS } from '../utils/colors';
import type { DFNode } from '../types';

type ContainerNodeData = {
  dfNode: DFNode;
};

export function ContainerNode({ data }: NodeProps) {
  const { dfNode } = data as ContainerNodeData;
  const statusColor = STATUS_COLORS[dfNode.status ?? 'stopped'] ?? STATUS_COLORS.stopped;
  const isGhost = dfNode.status === 'not_running';

  return (
    <div
      style={{
        background: '#1e293b',
        border: `1.5px ${isGhost ? 'dashed' : 'solid'} ${statusColor}`,
        borderRadius: 8,
        padding: '8px 12px',
        minWidth: 120,
        opacity: isGhost ? 0.5 : 1,
      }}
    >
      <Handle type="target" position={Position.Top} style={{ visibility: 'hidden' }} />

      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <span style={{ fontSize: 12, fontWeight: 600, color: '#e2e8f0' }}>
          {dfNode.name}
        </span>
        <span
          style={{
            width: 8,
            height: 8,
            borderRadius: '50%',
            background: statusColor,
            display: 'inline-block',
            transition: 'background 0.3s',
          }}
        />
      </div>

      {dfNode.image && (
        <div style={{ fontSize: 10, color: '#64748b', marginTop: 2 }}>
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
                color: '#94a3b8',
                background: '#0f172a',
                padding: '1px 4px',
                borderRadius: 3,
              }}
            >
              :{p.host} → {p.container}
            </span>
          ))}
        </div>
      )}

      <Handle type="source" position={Position.Bottom} style={{ visibility: 'hidden' }} />
    </div>
  );
}
