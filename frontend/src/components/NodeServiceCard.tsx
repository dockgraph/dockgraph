import { memo } from 'react';
import type { NodeProps } from '@xyflow/react';
import { NodeHandles } from './NodeHandles';
import { InspectButton } from './InspectButton';
import { STATUS_COLORS, cpuColor, stackColor } from '../utils/colors';
import { formatBytesShort } from '../utils/formatBytes';
import { ghostBorder, CARD_SHADOW, CARD_RADIUS } from '../utils/nodeStyles';
import { serviceCardHeight, CARD_WIDTH } from '../layout/nodeLayout';
import { useTheme } from '../theme';
import {
  INACTIVE_OPACITY,
  SERVICE_CARD_HEADER_HEIGHT,
  SERVICE_CARD_ROW_HEIGHT,
} from '../utils/constants';
import type { NodeServiceCardData, TaskInfo } from '../types';
import type { ContainerStatsData } from '../types/stats';

const ellipsis: React.CSSProperties = {
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap',
};

/** Colour of a task state dot. */
const TASK_STATE_COLORS: Record<string, string> = {
  running: STATUS_COLORS.running,
  complete: STATUS_COLORS.created,
  failed: STATUS_COLORS.exited,
  rejected: STATUS_COLORS.exited,
  shutdown: STATUS_COLORS.not_running,
};

/** Colour of a task state; transitional states (pending, starting...) read amber. */
function taskStateColor(state: string | undefined): string {
  return TASK_STATE_COLORS[state ?? ''] ?? STATUS_COLORS.unhealthy;
}

/** Row label of a task: `.{slot}` for replicated tasks, "global" otherwise. */
function slotLabel(task: TaskInfo): string {
  return task.slot ? `.${task.slot}` : 'global';
}

interface TaskRowProps {
  task: TaskInfo;
  serviceName: string;
  stats: ContainerStatsData | undefined;
  onOpen?: () => void;
}

/** One task inside a service card: slot, state dot, and live CPU/memory or state. */
const TaskRow = memo(function TaskRow({ task, serviceName, stats, onOpen }: TaskRowProps) {
  const { theme } = useTheme();
  const state = task.state ?? 'unknown';
  const running = state === 'running';
  const color = taskStateColor(state);
  const cpu = stats ? cpuColor(stats.cpuPercent, stats.cpuThrottled) : undefined;

  return (
    <div
      data-testid="task-row"
      role={onOpen ? 'button' : undefined}
      tabIndex={onOpen ? 0 : undefined}
      onClick={onOpen ? (e) => { e.stopPropagation(); onOpen(); } : undefined}
      onKeyDown={onOpen ? (e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); e.stopPropagation(); onOpen(); } } : undefined}
      title={`${serviceName}${task.slot ? `.${task.slot}` : ''} — ${state}${task.error ? `: ${task.error}` : ''}`}
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: 6,
        height: SERVICE_CARD_ROW_HEIGHT,
        padding: '0 6px',
        margin: '0 -6px',
        borderRadius: 4,
        minWidth: 0,
        opacity: running ? 1 : 0.8,
      }}
    >
      <span
        data-testid="task-state-dot"
        title={state}
        style={{
          width: 6,
          height: 6,
          borderRadius: '50%',
          background: running ? color : 'transparent',
          border: running ? 'none' : `1.5px solid ${color}`,
          boxSizing: 'border-box',
          flexShrink: 0,
        }}
      />
      <span
        style={{
          fontFamily: 'var(--dg-font-mono)',
          fontSize: 10.5,
          fontWeight: 600,
          color: theme.nodeText,
          width: 42,
          flexShrink: 0,
          ...ellipsis,
        }}
      >
        {slotLabel(task)}
      </span>
      {stats ? (
        <>
          <div
            title={`CPU: ${stats.cpuPercent.toFixed(1)}%`}
            style={{ flex: '0 0 34px', height: 3, background: theme.portBg, borderRadius: 1.5, overflow: 'hidden' }}
          >
            <div style={{ width: `${Math.min(stats.cpuPercent, 100)}%`, height: '100%', background: cpu, borderRadius: 1.5 }} />
          </div>
          <span style={{ ...ellipsis, minWidth: 0, fontFamily: 'var(--dg-font-mono)', fontSize: 9.5, color: theme.nodeSubtext }}>
            {stats.cpuPercent.toFixed(0)}%{stats.memUsage > 0 ? ` · ${formatBytesShort(stats.memUsage)}` : ''}
          </span>
        </>
      ) : (
        <span
          style={{
            ...ellipsis,
            minWidth: 0,
            fontSize: 9.5,
            color: task.error ? theme.danger : theme.nodeSubtext,
            fontStyle: running ? 'normal' : 'italic',
          }}
        >
          {task.error ?? state}
        </span>
      )}
    </div>
  );
});

/**
 * One service on one swarm node in the per-node view: the service name with
 * a running/placed badge, and one compact row per task the service has on the
 * node. Styled like the network view's cards (same shell, radius and shadow)
 * with the stack colour as the left stripe. Clicking the card or a task row
 * opens the service panel. `peerHover` outlines every card of the service
 * while one of them is hovered; cards on a down/drained/paused node are dimmed.
 */
export const NodeServiceCard = memo(function NodeServiceCard({ data }: NodeProps) {
  const {
    dgNode, serviceId, serviceName, stack, tasks, nodeInactive, taskStats, peerHover, nodeWidth = CARD_WIDTH, onInfoClick,
  } = data as unknown as NodeServiceCardData & { nodeWidth?: number };
  const { theme } = useTheme();
  const color = stackColor(stack);
  const running = tasks.filter((t) => t.state === 'running').length;
  const allRunning = running === tasks.length;
  const badgeColor = allRunning ? STATUS_COLORS.running : STATUS_COLORS.degraded;
  const open = onInfoClick ? () => onInfoClick(serviceId) : undefined;

  const shadow = running > 0 ? `${CARD_SHADOW}, inset 9px 0 18px -14px ${color}` : CARD_SHADOW;

  return (
    <div
      data-testid="node-service-card"
      data-peer-hover={peerHover ? 'true' : undefined}
      role={open ? 'button' : undefined}
      tabIndex={open ? 0 : undefined}
      onClick={open}
      onKeyDown={open ? (e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); open(); } } : undefined}
      title={`${serviceName}${stack ? ` (stack ${stack})` : ''} — ${running}/${tasks.length} running on this node`}
      style={{
        background: theme.nodeBg,
        ...ghostBorder(false, theme),
        borderLeft: `3px solid ${color}`,
        borderRadius: CARD_RADIUS,
        padding: '0 10px',
        width: nodeWidth,
        height: serviceCardHeight(tasks.length),
        boxSizing: 'border-box',
        overflow: 'hidden',
        opacity: nodeInactive ? INACTIVE_OPACITY : 1,
        cursor: open ? 'pointer' : 'default',
        boxShadow: peerHover ? `${shadow}, 0 0 0 1.5px ${theme.accent}` : shadow,
        transition: 'box-shadow 0.15s',
      }}
    >
      <NodeHandles />
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 6,
          height: SERVICE_CARD_HEADER_HEIGHT,
          minWidth: 0,
        }}
      >
        <span
          title={dgNode.name}
          style={{
            ...ellipsis,
            minWidth: 0,
            fontSize: 12,
            fontWeight: 600,
            fontFamily: 'var(--dg-font-mono)',
            letterSpacing: '-0.01em',
            color: theme.nodeText,
          }}
        >
          {serviceName}
        </span>
        <span style={{ flex: 1 }} />
        {open && (
          <InspectButton label={`Inspect ${serviceName}`} title="Inspect service" color={theme.nodeSubtext} onClick={open} />
        )}
        <span
          data-testid="card-running-badge"
          title={`${running} of ${tasks.length} task${tasks.length === 1 ? '' : 's'} running on this node`}
          style={{
            flexShrink: 0,
            fontFamily: 'var(--dg-font-mono)',
            fontSize: 10,
            fontWeight: 600,
            lineHeight: '14px',
            padding: '0 5px',
            borderRadius: 4,
            color: badgeColor,
            background: `${badgeColor}1f`,
            border: `1px solid ${badgeColor}55`,
            whiteSpace: 'nowrap',
          }}
        >
          {running}/{tasks.length}
        </span>
      </div>
      {tasks.map((task) => (
        <TaskRow
          key={task.id}
          task={task}
          serviceName={serviceName}
          stats={taskStats?.[task.id]}
          onOpen={open}
        />
      ))}
    </div>
  );
});
