import { memo } from 'react';
import type { NodeProps } from '@xyflow/react';
import { NodeHandles } from './NodeHandles';
import { InspectButton } from './InspectButton';
import { networkColor } from '../utils/colors';
import { useTheme } from '../theme';
import type { NetworkGroupData } from '../types';

export const NetworkGroup = memo(function NetworkGroup({ data }: NodeProps) {
  const { dgNode, onInfoClick } = data as unknown as NetworkGroupData;
  const { theme } = useTheme();
  const color = networkColor(dgNode.name);

  return (
    <div
      style={{
        width: '100%',
        height: '100%',
        boxSizing: 'border-box',
        border: `1px solid ${color}${theme.groupBorderAlpha}`,
        borderRadius: 10,
        background: `${color}${theme.groupBgAlpha}`,
        position: 'relative',
      }}
    >
      <NodeHandles />

      {/* Legend tab hanging from the top border: its top edge is collinear with
          the group's top border (square top corners), dropping into the box. */}
      <div
        style={{
          position: 'absolute',
          top: -1,
          left: 12,
          zIndex: 1,
          display: 'inline-flex',
          alignItems: 'center',
          gap: 5,
          maxWidth: 'calc(100% - 24px)',
          padding: '2px 8px',
          background: theme.canvasBg,
          border: `1px solid ${color}${theme.groupBorderAlpha}`,
          borderRadius: '0 0 6px 6px',
          fontFamily: 'var(--dg-font-mono)',
          fontSize: 10,
          lineHeight: 1.5,
          fontWeight: 600,
          color: `${color}${theme.groupTextAlpha}`,
          letterSpacing: '0.04em',
          textTransform: 'uppercase' as const,
        }}
      >
        {onInfoClick && (
          <InspectButton
            label={`Inspect ${dgNode.name}`}
            title="Inspect network"
            color={`${color}${theme.groupTextAlpha}`}
            onClick={() => onInfoClick(dgNode.id)}
          />
        )}
        <span
          aria-hidden="true"
          style={{ width: 6, height: 6, borderRadius: '50%', background: color, flex: '0 0 auto' }}
        />
        <span
          onClick={onInfoClick ? () => onInfoClick(dgNode.id) : undefined}
          title={dgNode.name}
          style={{
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
            minWidth: 0,
            ...(onInfoClick ? { cursor: 'pointer' } : null),
          }}
        >
          {dgNode.name}
        </span>
      </div>
    </div>
  );
});
