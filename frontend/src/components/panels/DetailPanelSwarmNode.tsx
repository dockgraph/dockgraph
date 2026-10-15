import { useMemo } from 'react';
import { useTheme } from '../../theme';
import { formatBytes } from '../../utils/formatBytes';
import { STATUS_COLORS, cpuColor, swarmNodeStateColor } from '../../utils/colors';
import { nodeStatsKey, taskContainerName } from '../../utils/stack';
import { placeTasks, nodeUsage, formatCores, taskLabel, type PlacedTask } from '../../utils/nodeTransform';
import { Section, Row, navLinkStyle, navButtonReset, monoStyle } from './shared';
import { Copyable } from './Copyable';
import { DetailPanelStats } from './DetailPanelStats';
import { SwarmRoleBadge, SwarmAvailabilityChip } from '../SwarmNodeBadges';
import type { DGNode } from '../../types';
import type { ContainerStatsData } from '../../types/stats';

/** Header for a swarm node: hostname, state, role and availability. */
export function DetailPanelSwarmNodeHeader({ node }: { node: DGNode }) {
  const { theme } = useTheme();
  const info = node.swarmNode;
  const state = info?.state ?? node.status ?? 'unknown';
  const color = swarmNodeStateColor(state);
  return (
    <div>
      <div style={{ fontSize: 16, fontWeight: 600, color: theme.nodeText, marginBottom: 6, wordBreak: 'break-all' }}>
        <Copyable value={node.name}>{node.name}</Copyable>
      </div>
      <div style={{ display: 'flex', alignItems: 'center', gap: 7, flexWrap: 'wrap' }}>
        <span
          style={{
            display: 'inline-flex',
            alignItems: 'center',
            gap: 6,
            padding: '3px 9px',
            borderRadius: 999,
            background: `${color}22`,
            fontFamily: 'var(--dg-font-mono)',
            fontSize: 10,
            fontWeight: 600,
            letterSpacing: '0.04em',
            textTransform: 'uppercase',
            color,
          }}
        >
          <span style={{ width: 6, height: 6, borderRadius: '50%', background: color, flexShrink: 0 }} />
          {state}
        </span>
        <SwarmRoleBadge info={info} />
        <SwarmAvailabilityChip info={info} />
        <span style={{ fontFamily: 'var(--dg-font-mono)', fontSize: 11, color: theme.nodeSubtext }}>swarm node</span>
      </div>
    </div>
  );
}

/** Colour of a task state dot. */
const TASK_STATE_COLORS: Record<string, string> = {
  running: STATUS_COLORS.running,
  complete: STATUS_COLORS.created,
  failed: STATUS_COLORS.exited,
  rejected: STATUS_COLORS.exited,
  shutdown: STATUS_COLORS.not_running,
};

interface Props {
  node: DGNode;
  /** The (stack-scoped) graph, used to find the tasks placed on the node. */
  dgNodes: DGNode[];
  /** Live stats keyed by workload name, plus `node:{hostname}` aggregates. */
  statsMap: Map<string, ContainerStatsData>;
  onNavigate: (targetId: string) => void;
}

/**
 * Detail panel body for a swarm node: live aggregate stats, node facts
 * (role, availability, address, engine), capacity with usage bars, and the
 * tasks placed on it grouped by service. Each service links to its panel.
 */
export function DetailPanelSwarmNode({ node, dgNodes, statsMap, onNavigate }: Props) {
  const { theme } = useTheme();
  const mono = monoStyle(theme.panelText);
  const info = node.swarmNode;
  const stats = statsMap.get(nodeStatsKey(node.name));
  const usage = nodeUsage(info, stats);

  // Tasks on this node, grouped by service (placeTasks already sorts them).
  const byService = useMemo(() => {
    const groups: { service: DGNode; tasks: PlacedTask[] }[] = [];
    for (const pt of placeTasks(dgNodes).get(node.id) ?? []) {
      const last = groups[groups.length - 1];
      if (last && last.service.id === pt.service.id) last.tasks.push(pt);
      else groups.push({ service: pt.service, tasks: [pt] });
    }
    return groups;
  }, [dgNodes, node.id]);
  const taskCount = byService.reduce((n, g) => n + g.tasks.length, 0);

  return (
    <>
      {stats ? (
        <DetailPanelStats stats={stats} />
      ) : (
        <div style={{ fontSize: 11, color: theme.nodeSubtext, marginBottom: 14, fontStyle: 'italic' }}>
          No agent reports stats for this node.
        </div>
      )}

      <Section title="Node">
        {info && <Row label="Role" value={info.leader ? `${info.role} (leader)` : info.role} mono={mono} subtext={theme.nodeSubtext} />}
        {info && <Row label="Availability" value={info.availability} mono={mono} subtext={theme.nodeSubtext} />}
        <Row label="State" value={info?.state ?? node.status ?? 'unknown'} mono={mono} subtext={theme.nodeSubtext} />
        {info?.addr && <Row label="Address" value={info.addr} mono={mono} subtext={theme.nodeSubtext} />}
        {info?.engineVersion && <Row label="Engine" value={info.engineVersion} mono={mono} subtext={theme.nodeSubtext} />}
        {info?.id && <Row label="ID" value={info.id} mono={mono} subtext={theme.nodeSubtext} />}
      </Section>

      {(usage.cores !== undefined || !!info?.memoryBytes) && (
        <Section title="Resources">
          {usage.cores !== undefined && (
            <ResourceLine
              label="CPU"
              capacity={formatCores(usage.cores)}
              percent={usage.cpuPercent}
              color={cpuColor(usage.cpuPercent ?? 0, 0)}
            />
          )}
          {info?.memoryBytes ? (
            <ResourceLine
              label="Memory"
              capacity={formatBytes(info.memoryBytes)}
              used={stats ? formatBytes(stats.memUsage) : undefined}
              percent={usage.memPercent}
              color={theme.info}
            />
          ) : null}
        </Section>
      )}

      <Section title={`Tasks (${taskCount})`}>
        {byService.length === 0 ? (
          <div style={{ fontSize: 11, color: theme.nodeSubtext }}>No tasks on this node.</div>
        ) : (
          byService.map(({ service, tasks }) => (
            <div key={service.id} style={{ marginBottom: 8 }}>
              <div style={{ fontSize: 11, fontWeight: 600, color: theme.nodeText, marginBottom: 3 }}>
                <button
                  type="button"
                  style={{ ...navButtonReset, fontWeight: 600, fontFamily: 'var(--dg-font-mono)', ...navLinkStyle(theme.panelBorder) }}
                  title={`Inspect service ${service.name}`}
                  onClick={() => onNavigate(service.id)}
                >
                  {service.name}
                </button>
              </div>
              {tasks.map(({ task }) => {
                const s = statsMap.get(taskContainerName(service.name, task));
                const color = TASK_STATE_COLORS[task.state ?? ''] ?? STATUS_COLORS.unhealthy;
                return (
                  <div
                    key={task.id}
                    style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 11, color: theme.panelText, padding: '2px 0 2px 10px' }}
                  >
                    <span style={{ width: 6, height: 6, borderRadius: '50%', background: color, flexShrink: 0 }} />
                    <span style={{ fontFamily: 'var(--dg-font-mono)', flex: 1, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                      {taskLabel(service.name, task)}
                    </span>
                    <span style={{ color: theme.nodeSubtext }}>{task.state ?? 'unknown'}</span>
                    <span style={{ fontFamily: 'var(--dg-font-mono)', fontSize: 10.5, color: theme.nodeSubtext, whiteSpace: 'nowrap' }}>
                      {s ? `${s.cpuPercent.toFixed(1)}% · ${formatBytes(s.memUsage)}` : '—'}
                    </span>
                  </div>
                );
              })}
            </div>
          ))
        )}
      </Section>
    </>
  );
}

function ResourceLine({ label, capacity, used, percent, color }: { label: string; capacity: string; used?: string; percent?: number; color: string }) {
  const { theme } = useTheme();
  return (
    <div style={{ marginBottom: 7 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 11, marginBottom: 3 }}>
        <span style={{ color: theme.nodeSubtext }}>{label}</span>
        <span style={{ fontFamily: 'var(--dg-font-mono)', color: theme.panelText }}>
          {percent !== undefined ? `${percent.toFixed(0)}% · ` : ''}{used ? `${used} / ` : ''}{capacity}
        </span>
      </div>
      <div style={{ height: 4, background: theme.portBg, borderRadius: 2, overflow: 'hidden' }}>
        <div style={{ width: `${percent ?? 0}%`, height: '100%', background: color, borderRadius: 2 }} />
      </div>
    </div>
  );
}
