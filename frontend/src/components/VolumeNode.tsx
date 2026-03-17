import { Handle, Position, type NodeProps } from '@xyflow/react';
import type { DFNode } from '../types';

type VolumeNodeData = {
  dfNode: DFNode;
};

export function VolumeNode({ data }: NodeProps) {
  const { dfNode } = data as VolumeNodeData;

  return (
    <div
      style={{
        background: '#1e293b',
        border: '1.5px solid #f97316',
        borderRadius: 8,
        padding: '6px 10px',
        display: 'flex',
        alignItems: 'center',
        gap: 6,
      }}
    >
      <Handle type="source" position={Position.Top} style={{ visibility: 'hidden' }} />

      <span style={{ fontSize: 14 }}>💾</span>
      <div>
        <div style={{ fontSize: 11, fontWeight: 600, color: '#e2e8f0' }}>
          {dfNode.name}
        </div>
        {dfNode.driver && (
          <div style={{ fontSize: 9, color: '#64748b' }}>{dfNode.driver}</div>
        )}
      </div>

      <Handle type="target" position={Position.Bottom} style={{ visibility: 'hidden' }} />
    </div>
  );
}
