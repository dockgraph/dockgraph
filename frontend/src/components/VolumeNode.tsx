import type { NodeProps } from '@xyflow/react';
import { NodeHandles } from './NodeHandles';
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
        border: '1px solid #334155',
        borderLeft: '3px solid #f97316',
        borderRadius: 4,
        padding: '5px 10px',
        display: 'flex',
        alignItems: 'center',
        gap: 6,
        width: 196,
      }}
    >
      <NodeHandles />

      <span style={{ fontSize: 14 }}>💾</span>
      <div style={{ overflow: 'hidden' }}>
        <div
          style={{
            fontSize: 11,
            fontWeight: 600,
            color: '#e2e8f0',
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
          }}
          title={dfNode.name}
        >
          {dfNode.name}
        </div>
        {dfNode.driver && (
          <div style={{ fontSize: 9, color: '#64748b' }}>{dfNode.driver}</div>
        )}
      </div>

    </div>
  );
}
