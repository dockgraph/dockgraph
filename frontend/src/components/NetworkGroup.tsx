import type { NodeProps } from '@xyflow/react';
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
        border: `2px solid ${color}`,
        borderRadius: 10,
        background: `${color}08`,
        position: 'relative',
      }}
    >
      <div
        style={{
          padding: '4px 10px',
          fontSize: 11,
          fontWeight: 600,
          color,
          borderBottom: `1px solid ${color}30`,
        }}
      >
        {dfNode.name}
      </div>
    </div>
  );
}
