import { memo } from 'react';
import type { NodeProps } from '@xyflow/react';
import { useStore } from '@xyflow/react';
import { NodeHandles } from './NodeHandles';
import { InspectButton } from './InspectButton';
import { StatsMini } from './StatsMini';
import { STATUS_COLORS, STATUS_LABELS } from '../utils/colors';
import { useTheme } from '../theme';
import { ghostBorder, railColor } from '../utils/nodeStyles';
import { CONTAINER_NODE_HEIGHT, INACTIVE_OPACITY, zoomSelector } from '../utils/constants';
import { isServiceLive } from '../utils/stack';
import type { ServiceNodeData } from '../types';

const ellipsis: React.CSSProperties = {
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap',
};

/** Short chip text for a swarm service mode. */
function modeLabel(mode: string | undefined): string | null {
  switch (mode) {
    case 'replicated': return 'repl';
    case 'global': return 'global';
    case 'replicated-job': return 'job';
    case 'global-job': return 'global job';
    default: return null;
  }
}

/**
 * Graph node for a swarm service. Mirrors ContainerNode's shell, and adds a
 * running/desired replicas badge, a mode chip and a status rail that turns
 * amber when the service is degraded. The shell stacks a faint second card
 * behind the node to read as "many tasks" rather than a single container.
 */
export const ServiceNode = memo(function ServiceNode({ data }: NodeProps) {
  const { dgNode, nodeWidth, stats, onInfoClick } = data as unknown as ServiceNodeData;
  const w = nodeWidth ?? 200;
  const { theme } = useTheme();
  const isLowZoom = useStore(zoomSelector);
  const status = dgNode.status ?? 'stopped';
  const statusColor = STATUS_COLORS[status] ?? STATUS_COLORS.exited;
  const isGhost = status === 'not_running';
  const rail = railColor(isGhost, statusColor);
  const isActive = isServiceLive(dgNode);
  const replicas = dgNode.service?.replicas;
  const mode = modeLabel(dgNode.service?.mode);
  const degraded = status === 'degraded';

  const shell: React.CSSProperties = {
    background: theme.nodeBg,
    ...ghostBorder(isGhost, theme),
    borderLeft: `3px solid ${rail}`,
    borderRadius: 6,
    padding: '7px 10px',
    width: w,
    height: CONTAINER_NODE_HEIGHT,
    boxSizing: 'border-box',
    overflow: 'hidden',
    opacity: isActive ? 1 : INACTIVE_OPACITY,
    boxShadow: isActive
      ? `3px 3px 0 -1px ${theme.nodeBg}, 3px 3px 0 0 ${theme.nodeBorder}, inset 9px 0 18px -14px ${statusColor}`
      : `3px 3px 0 -1px ${theme.nodeBg}, 3px 3px 0 0 ${theme.nodeBorder}`,
  };

  const replicaBadge = replicas && (
    <span
      data-testid="replicas-badge"
      title={`${replicas.running} of ${replicas.desired} replicas running — ${STATUS_LABELS[status] ?? status}`}
      style={{
        fontFamily: 'var(--dg-font-mono)',
        fontSize: 10,
        fontWeight: 600,
        lineHeight: '14px',
        padding: '0 5px',
        borderRadius: 4,
        color: statusColor,
        background: `${statusColor}${degraded ? '33' : '1f'}`,
        border: `1px solid ${statusColor}${degraded ? 'aa' : '55'}`,
        whiteSpace: 'nowrap',
      }}
    >
      {replicas.running}/{replicas.desired}
    </span>
  );

  if (isLowZoom) {
    return (
      <div style={shell}>
        <NodeHandles />
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 6 }}>
          <span style={{ ...ellipsis, fontSize: 12, fontWeight: 600, fontFamily: 'var(--dg-font-mono)', color: theme.nodeText }}>
            {dgNode.name}
          </span>
          {replicaBadge}
        </div>
      </div>
    );
  }

  return (
    <div style={shell}>
      <NodeHandles />

      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 6 }}>
        <span
          style={{
            ...ellipsis,
            fontSize: 12.5,
            fontWeight: 600,
            fontFamily: 'var(--dg-font-mono)',
            letterSpacing: '-0.01em',
            color: theme.nodeText,
            minWidth: 0,
          }}
          title={dgNode.name}
        >
          {dgNode.name}
        </span>
        <div style={{ display: 'flex', alignItems: 'center', gap: 4, flexShrink: 0 }}>
          {onInfoClick && (
            <InspectButton
              label={`Inspect ${dgNode.name}`}
              title="Inspect service"
              color={theme.nodeSubtext}
              onClick={() => onInfoClick(dgNode.id)}
            />
          )}
          {replicaBadge}
        </div>
      </div>

      <div style={{ display: 'flex', alignItems: 'center', gap: 5, marginTop: 3, minWidth: 0 }}>
        {mode && (
          <span
            data-testid="mode-chip"
            title={`Mode: ${dgNode.service?.mode}`}
            style={{
              flexShrink: 0,
              fontFamily: 'var(--dg-font-mono)',
              fontSize: 9,
              fontWeight: 600,
              textTransform: 'uppercase',
              letterSpacing: '0.05em',
              color: theme.nodeSubtext,
              border: `1px solid ${theme.panelBorder}`,
              borderRadius: 3,
              padding: '0 4px',
              lineHeight: '13px',
            }}
          >
            {mode}
          </span>
        )}
        {dgNode.image && (
          <span
            style={{ ...ellipsis, fontFamily: 'var(--dg-font-mono)', fontSize: 10, color: theme.nodeSubtext, minWidth: 0 }}
            title={dgNode.image}
          >
            {dgNode.image}
          </span>
        )}
      </div>

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
              {p.host ? <>:{p.host}&#8201;&rarr;&#8201;</> : null}{p.container}
            </span>
          ))}
          {dgNode.ports.length > 3 && (
            <span style={{ fontFamily: 'var(--dg-font-mono)', fontSize: 9.5, color: theme.nodeSubtext, alignSelf: 'center' }}>
              +{dgNode.ports.length - 3}
            </span>
          )}
        </div>
      )}

      <StatsMini stats={stats} />
    </div>
  );
});
