import { memo } from 'react';
import type { NodeProps } from '@xyflow/react';
import { useStore } from '@xyflow/react';
import { NodeHandles } from './NodeHandles';
import { InspectButton } from './InspectButton';
import { StatsMini } from './StatsMini';
import { STATUS_COLORS, STATUS_LABELS } from '../utils/colors';
import { useTheme } from '../theme';
import { ghostBorder } from '../utils/nodeStyles';
import { CONTAINER_NODE_HEIGHT, STATUS_DOT_SIZE, INACTIVE_OPACITY, PAUSED_OPACITY, zoomSelector } from '../utils/constants';
import type { ContainerNodeData } from '../types';

const ellipsis: React.CSSProperties = {
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap',
};

export const ContainerNode = memo(function ContainerNode({ data }: NodeProps) {
  const { dgNode, nodeWidth, stats, onInfoClick } = data as unknown as ContainerNodeData;
  const w = nodeWidth ?? 200;
  const { theme } = useTheme();
  const isLowZoom = useStore(zoomSelector);
  const statusColor = STATUS_COLORS[dgNode.status ?? 'exited'] ?? STATUS_COLORS.exited;
  const isGhost = dgNode.status === 'not_running';
  const isActive = dgNode.status === 'running' || dgNode.status === 'unhealthy';
  const isPaused = dgNode.status === 'paused';
  const opacity = isActive ? 1 : isPaused ? PAUSED_OPACITY : INACTIVE_OPACITY;

  // Shared shell: 1px outline (3 sides), a glowing status rail on the left,
  // and a faint lift. Active containers get an inner glow bled from the rail.
  const shell: React.CSSProperties = {
    background: theme.nodeBg,
    ...ghostBorder(isGhost, theme),
    borderLeft: `3px solid ${statusColor}`,
    borderRadius: 6,
    padding: '7px 10px',
    width: w,
    height: CONTAINER_NODE_HEIGHT,
    boxSizing: 'border-box',
    overflow: 'hidden',
    opacity,
    boxShadow: isActive
      ? `0 2px 6px -3px rgba(0, 0, 0, 0.45), inset 9px 0 18px -14px ${statusColor}`
      : '0 2px 6px -3px rgba(0, 0, 0, 0.45)',
  };

  // Simplified render at low zoom — just a colored block with the name.
  if (isLowZoom) {
    return (
      <div style={shell}>
        <NodeHandles />
        <span style={{ ...ellipsis, fontSize: 12, fontWeight: 600, fontFamily: 'var(--dg-font-mono)', color: theme.nodeText, display: 'block' }}>
          {dgNode.name}
        </span>
      </div>
    );
  }

  return (
    <div style={shell}>
      <NodeHandles />

      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <span
          style={{
            ...ellipsis,
            fontSize: 12.5,
            fontWeight: 600,
            fontFamily: 'var(--dg-font-mono)',
            letterSpacing: '-0.01em',
            color: theme.nodeText,
            maxWidth: 'calc(100% - 30px)',
            display: 'inline-block',
          }}
          title={dgNode.name}
        >
          {dgNode.name}
        </span>
        <div style={{ display: 'flex', alignItems: 'center', gap: 4, flexShrink: 0 }}>
          {onInfoClick && (
            <InspectButton
              label={`Inspect ${dgNode.name}`}
              title="Inspect container"
              color={theme.nodeSubtext}
              onClick={() => onInfoClick(dgNode.id)}
            />
          )}
          <span
            role="img"
            aria-label={STATUS_LABELS[dgNode.status ?? 'exited'] ?? dgNode.status}
            title={STATUS_LABELS[dgNode.status ?? 'exited'] ?? dgNode.status}
            style={{
              width: STATUS_DOT_SIZE,
              height: STATUS_DOT_SIZE,
              borderRadius: '50%',
              background: isActive ? statusColor : 'transparent',
              border: isActive ? 'none' : `1.5px solid ${statusColor}`,
              boxShadow: isActive ? `0 0 6px ${statusColor}` : 'none',
              display: 'inline-block',
              boxSizing: 'border-box',
              transition: 'background 0.3s, border-color 0.3s',
            }}
          />
        </div>
      </div>

      {dgNode.image && (
        <div
          style={{
            ...ellipsis,
            fontFamily: 'var(--dg-font-mono)',
            fontSize: 10,
            color: theme.nodeSubtext,
            marginTop: 3,
          }}
          title={dgNode.image}
        >
          {dgNode.image}
        </div>
      )}

      {dgNode.ports && dgNode.ports.length > 0 && (
        <div style={{ marginTop: 5, display: 'flex', gap: 4, flexWrap: 'wrap' }}>
          {dgNode.ports.slice(0, 3).map((p, i) => (
            <span
              key={`${i}-${p.host}-${p.container}`}
              style={{
                fontFamily: 'var(--dg-font-mono)',
                fontSize: 9.5,
                color: theme.portText,
                background: theme.portBg,
                border: `1px solid ${theme.panelBorder}`,
                padding: '0px 5px',
                borderRadius: 4,
                whiteSpace: 'nowrap',
              }}
            >
              :{p.host}&#8201;&rarr;&#8201;{p.container}
            </span>
          ))}
          {dgNode.ports.length > 3 && (
            <span
              title={dgNode.ports.slice(3).map((p) => `:${p.host} → ${p.container}`).join(', ')}
              style={{ fontFamily: 'var(--dg-font-mono)', fontSize: 9.5, color: theme.nodeSubtext, alignSelf: 'center' }}
            >
              +{dgNode.ports.length - 3}
            </span>
          )}
        </div>
      )}

      <StatsMini stats={stats} />
    </div>
  );
});
