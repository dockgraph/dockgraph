import { memo, useCallback } from 'react';
import type { NodeProps } from '@xyflow/react';
import { NodeHandles } from './NodeHandles';
import { networkColor } from '../utils/colors';
import { useTheme } from '../theme';
import type { NetworkGroupData } from '../types';

export const NetworkGroup = memo(function NetworkGroup({ data }: NodeProps) {
  const { dgNode, onInfoClick } = data as unknown as NetworkGroupData;
  const { theme } = useTheme();
  const color = networkColor(dgNode.name);

  const handleInfo = useCallback(
    (e: React.MouseEvent) => {
      e.stopPropagation();
      onInfoClick?.(dgNode.id);
    },
    [onInfoClick, dgNode.id],
  );

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
          display: 'flex',
          alignItems: 'center',
          gap: 4,
        }}
      >
        <button
          onClick={handleInfo}
          aria-label={`Inspect ${dgNode.name}`}
          title="Inspect network"
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
          }}
        >
          <span style={{ width: 10, height: 2, background: `${color}${theme.groupTextAlpha}`, borderRadius: 1, display: 'block' }} />
          <span style={{ width: 10, height: 2, background: `${color}${theme.groupTextAlpha}`, borderRadius: 1, display: 'block' }} />
          <span style={{ width: 7, height: 2, background: `${color}${theme.groupTextAlpha}`, borderRadius: 1, display: 'block' }} />
        </button>
        <span onClick={handleInfo} style={{ cursor: 'pointer' }}>{dgNode.name}</span>
      </div>

    </div>
  );
});
