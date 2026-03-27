import { memo } from 'react';
import type { NodeProps } from '@xyflow/react';
import { NodeHandles } from './NodeHandles';
import { networkColor } from '../utils/colors';
import { useTheme } from '../theme';
import type { NetworkGroupData } from '../types';

export const NetworkGroup = memo(function NetworkGroup({ data }: NodeProps) {
  const { dgNode } = data as unknown as NetworkGroupData;
  const { theme } = useTheme();
  const color = networkColor(dgNode.name);

  return (
    <div
      style={{
        width: '100%',
        height: '100%',
        boxSizing: 'border-box',
        border: `1px solid ${color}${theme.groupBorderAlpha}`,
        borderRadius: 6,
        background: `${color}${theme.groupBgAlpha}`,
        position: 'relative',
      }}
    >
      <NodeHandles />

      <div
        style={{
          padding: '3px 8px',
          fontSize: 10,
          fontWeight: 600,
          color: `${color}${theme.groupTextAlpha}`,
          letterSpacing: '0.3px',
          textTransform: 'uppercase' as const,
        }}
      >
        {dgNode.name}
      </div>

    </div>
  );
});
