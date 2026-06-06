import { memo } from 'react';
import type { NodeProps } from '@xyflow/react';
import { useStore } from '@xyflow/react';
import { NodeHandles } from './NodeHandles';
import { InspectButton } from './InspectButton';
import { useTheme } from '../theme';
import { VOLUME_COLOR } from '../utils/colors';
import { ghostBorder } from '../utils/nodeStyles';
import type { VolumeNodeData } from '../types';
import { INACTIVE_OPACITY, zoomSelector } from '../utils/constants';

function VolumeIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke={VOLUME_COLOR} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <ellipse cx="12" cy="5" rx="8" ry="3" />
      <path d="M4 5v14c0 1.66 3.58 3 8 3s8-1.34 8-3V5" />
      <path d="M4 12c0 1.66 3.58 3 8 3s8-1.34 8-3" />
    </svg>
  );
}

export const VolumeNode = memo(function VolumeNode({ data }: NodeProps) {
  const { dgNode, nodeWidth, onInfoClick } = data as unknown as VolumeNodeData;
  const w = nodeWidth ?? 200;
  const { theme } = useTheme();
  const isLowZoom = useStore(zoomSelector);
  const isGhost = dgNode.status === 'not_running';
  const opacity = isGhost ? INACTIVE_OPACITY : 1;

  const shell: React.CSSProperties = {
    background: theme.nodeBg,
    ...ghostBorder(isGhost, theme),
    borderLeft: `3px solid ${VOLUME_COLOR}`,
    borderRadius: 6,
    width: w,
    height: 40,
    boxSizing: 'border-box',
    opacity,
    overflow: 'hidden',
    boxShadow: '0 2px 6px -3px rgba(0, 0, 0, 0.45)',
  };

  if (isLowZoom) {
    return (
      <div style={{ ...shell, padding: '5px 10px' }}>
        <NodeHandles />
      </div>
    );
  }

  return (
    <div
      style={{
        ...shell,
        padding: '5px 10px',
        display: 'flex',
        alignItems: 'center',
        gap: 7,
      }}
    >
      <NodeHandles />

      <VolumeIcon />
      <div style={{ overflow: 'hidden', flex: 1, minWidth: 0 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <div
            style={{
              fontFamily: 'var(--dg-font-mono)',
              fontSize: 11.5,
              fontWeight: 600,
              letterSpacing: '-0.01em',
              color: theme.nodeText,
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap',
            }}
            title={dgNode.name}
          >
            {dgNode.name}
          </div>
          {onInfoClick && (
            <InspectButton
              label={`Inspect ${dgNode.name}`}
              title="Inspect volume"
              color={theme.nodeSubtext}
              onClick={() => onInfoClick(dgNode.id)}
            />
          )}
        </div>
        {dgNode.driver && (
          <div style={{ fontFamily: 'var(--dg-font-mono)', fontSize: 9, color: theme.nodeSubtext }}>{dgNode.driver}</div>
        )}
      </div>
    </div>
  );
});
