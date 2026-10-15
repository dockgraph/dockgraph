import { memo } from 'react';
import type { NodeProps } from '@xyflow/react';
import { NodeHandles } from './NodeHandles';
import { InspectButton } from './InspectButton';
import { SwarmRoleBadge, SwarmAvailabilityChip, SwarmStateChip } from './SwarmNodeBadges';
import { cpuColor, swarmNodeStateColor, swarmRoleColor } from '../utils/colors';
import { formatBytesShort } from '../utils/formatBytes';
import { nodeUsage, formatCores } from '../utils/nodeTransform';
import { NODE_BOX_HEADER_HEIGHT } from '../utils/constants';
import { useTheme } from '../theme';
import type { SwarmNodeGroupData } from '../types';

const ellipsis: React.CSSProperties = {
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap',
};

interface MiniBarProps {
  label: string;
  percent: number | undefined;
  color: string;
  text: string;
  title: string;
}

/** Captioned resource bar for the node header (CPU or memory against capacity). */
function MiniBar({ label, percent, color, text, title }: MiniBarProps) {
  const { theme } = useTheme();
  return (
    <div title={title} style={{ display: 'flex', alignItems: 'center', gap: 5, flex: 1, minWidth: 0 }}>
      <span style={{ fontFamily: 'var(--dg-font-mono)', fontSize: 9, fontWeight: 600, color: theme.nodeSubtext, letterSpacing: '0.05em' }}>
        {label}
      </span>
      <div style={{ flex: 1, minWidth: 24, height: 4, background: theme.portBg, borderRadius: 2, overflow: 'hidden' }}>
        <div style={{ width: `${percent ?? 0}%`, height: '100%', background: color, borderRadius: 2 }} />
      </div>
      <span style={{ ...ellipsis, fontFamily: 'var(--dg-font-mono)', fontSize: 9.5, color: theme.nodeSubtext, flexShrink: 0 }}>
        {text}
      </span>
    </div>
  );
}

/**
 * Box of the per-node graph view: one swarm node with its service cards
 * inside. The header shows hostname, role (★ for the leader), a state chip
 * when not ready, availability when not active, and CPU/memory bars of the
 * node's aggregate usage against its capacity; a stripe in the role colour
 * tops it. Down, drained and paused nodes are dimmed. The Unassigned box
 * holds tasks the scheduler couldn't place.
 */
export const SwarmNodeGroup = memo(function SwarmNodeGroup({ data }: NodeProps) {
  const { dgNode, unassigned, role, taskCount, stats, onInfoClick } = data as unknown as SwarmNodeGroupData;
  const { theme } = useTheme();
  const info = dgNode.swarmNode;
  const state = info?.state ?? dgNode.status;
  const stateColor = unassigned ? theme.warning : swarmNodeStateColor(state);
  const down = !unassigned && state !== 'ready';
  const inactive = down || info?.availability === 'drain' || info?.availability === 'pause';
  const accent = role ? swarmRoleColor(role, theme.mode) : undefined;
  const usage = nodeUsage(info, stats);
  const openInfo = onInfoClick && !unassigned ? () => onInfoClick(dgNode.id) : undefined;

  const cpuText = usage.cpuPercent !== undefined
    ? `${usage.cpuPercent.toFixed(0)}%`
    : stats ? `${stats.cpuPercent.toFixed(0)}%` : '—';
  const memText = stats ? formatBytesShort(stats.memUsage) : '—';

  return (
    <div
      data-testid="swarm-node-group"
      style={{
        width: '100%',
        height: '100%',
        boxSizing: 'border-box',
        border: `1px ${unassigned ? 'dashed' : 'solid'} ${down ? stateColor : theme.nodeGhostBorder}`,
        borderRadius: 10,
        background: `${theme.panelBg}${theme.mode === 'dark' ? 'cc' : 'e6'}`,
        position: 'relative',
        opacity: inactive ? 0.7 : 1,
      }}
    >
      <NodeHandles />

      <div
        style={{
          position: 'absolute',
          top: 0,
          left: 0,
          right: 0,
          padding: '8px 12px 7px',
          borderBottom: `1px solid ${theme.panelBorder}`,
          borderRadius: '10px 10px 0 0',
          background: theme.nodeBg,
          ...(accent ? { boxShadow: `inset 0 2px 0 ${accent}` } : null),
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: 6, minWidth: 0 }}>
          <span
            data-testid="node-state-dot"
            title={unassigned ? 'Not placed on a node' : `State: ${info?.state ?? dgNode.status ?? 'unknown'}`}
            style={{ width: 7, height: 7, borderRadius: '50%', background: stateColor, flexShrink: 0 }}
          />
          <span
            onClick={openInfo}
            title={dgNode.name}
            style={{
              ...ellipsis,
              minWidth: 0,
              fontFamily: 'var(--dg-font-mono)',
              fontSize: 12.5,
              fontWeight: 600,
              color: theme.nodeText,
              fontStyle: unassigned ? 'italic' : 'normal',
              ...(openInfo ? { cursor: 'pointer' } : null),
            }}
          >
            {dgNode.name}
          </span>
          <SwarmRoleBadge info={info} />
          {!unassigned && <SwarmStateChip state={state} />}
          <SwarmAvailabilityChip info={info} />
          <span style={{ flex: 1 }} />
          <span
            data-testid="task-count"
            title={`${taskCount} task${taskCount === 1 ? '' : 's'}`}
            style={{ fontFamily: 'var(--dg-font-mono)', fontSize: 10, color: theme.nodeSubtext, flexShrink: 0 }}
          >
            {taskCount} {taskCount === 1 ? 'task' : 'tasks'}
          </span>
          {openInfo && (
            <InspectButton label={`Inspect ${dgNode.name}`} title="Inspect node" color={theme.nodeSubtext} onClick={openInfo} />
          )}
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginTop: 7, height: 14 }}>
          {unassigned ? (
            <span style={{ fontSize: 10, color: theme.nodeSubtext }}>Tasks waiting for a node</span>
          ) : stats ? (
            <>
              <MiniBar
                label="CPU"
                percent={usage.cpuPercent}
                color={cpuColor(usage.cpuPercent ?? 0, 0)}
                text={cpuText}
                title={`CPU: ${stats.cpuPercent.toFixed(1)}%${usage.cores ? ` of ${formatCores(usage.cores)}` : ''}`}
              />
              <MiniBar
                label="MEM"
                percent={usage.memPercent}
                color={theme.info}
                text={memText}
                title={`Memory: ${formatBytesShort(stats.memUsage)}${info?.memoryBytes ? ` of ${formatBytesShort(info.memoryBytes)}` : ''}`}
              />
            </>
          ) : (
            <span data-testid="no-agent" style={{ fontSize: 10, color: theme.nodeSubtext, fontStyle: 'italic' }}>
              no agent{usage.cores ? ` · ${formatCores(usage.cores)}` : ''}{info?.memoryBytes ? ` · ${formatBytesShort(info.memoryBytes)}` : ''}
            </span>
          )}
        </div>
      </div>

      {taskCount === 0 && (
        <div
          data-testid="no-tasks"
          style={{
            position: 'absolute',
            top: NODE_BOX_HEADER_HEIGHT,
            left: 0,
            right: 0,
            bottom: 0,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            fontSize: 10.5,
            fontStyle: 'italic',
            color: theme.nodeSubtext,
          }}
        >
          No tasks
        </div>
      )}
    </div>
  );
});
