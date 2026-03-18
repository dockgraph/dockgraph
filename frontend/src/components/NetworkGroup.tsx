import type { NodeProps } from '@xyflow/react';
import { NodeHandles } from './NodeHandles';
import { networkColor } from '../utils/colors';
import type { DFNode } from '../types';

type NetworkGroupData = {
  dfNode: DFNode;
};

export function NetworkGroup({ data }: NodeProps) {
  const { dfNode } = data as NetworkGroupData;
  const color = networkColor(dfNode.name);

  return (
    <div
      style={{
        width: '100%',
        height: '100%',
        border: `1px solid ${color}40`,
        borderRadius: 6,
        background: `${color}06`,
        position: 'relative',
      }}
    >
      <NodeHandles />

      <div
        style={{
          padding: '3px 8px',
          fontSize: 10,
          fontWeight: 600,
          color: `${color}cc`,
          letterSpacing: '0.3px',
          textTransform: 'uppercase' as const,
        }}
      >
        {dfNode.name}
      </div>

    </div>
  );
}
