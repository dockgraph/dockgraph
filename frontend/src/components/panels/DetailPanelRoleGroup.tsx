import type { ReactNode } from 'react';
import { useTheme } from '../../theme';
import { formatBytes } from '../../utils/formatBytes';
import { formatCount } from '../../utils/format';
import { cpuColor, swarmNodeStateColor, swarmRoleColor } from '../../utils/colors';
import { formatCores, roleTitle } from '../../utils/nodeTransform';
import { quorumHealth, type RoleSummary, type Tally } from '../../utils/roleSummary';
import { useSwarmManagers } from '../../hooks/useSwarmManagers';
import { Section, Row, navLinkStyle, navButtonReset, monoStyle } from './shared';
import { panelTableStyles } from './panelStyles';
import { ResourceLine } from './ResourceLine';
import { Pill } from './StatusBadge';
import type { SwarmRole } from '../../types';

/** "14 ready · 1 down". */
const tallyText = (tally: Tally) => tally.map(({ value, count }) => `${count} ${value}`).join(' · ');

/** Header for a Managers / Workers group: title in the role colour, node and task counts. */
export function DetailPanelRoleGroupHeader({ role, summary }: { role: SwarmRole; summary: RoleSummary }) {
  const { theme } = useTheme();
  return (
    <div>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 16, fontWeight: 600, color: theme.nodeText, marginBottom: 4 }}>
        <span aria-hidden="true" style={{ width: 8, height: 8, borderRadius: '50%', background: swarmRoleColor(role, theme.mode), flexShrink: 0 }} />
        {roleTitle(role)}
      </div>
      <div style={{ fontFamily: 'var(--dg-font-mono)', fontSize: 11, color: theme.nodeSubtext }}>
        {`swarm ${role}s · ${formatCount(summary.members.length, 'node')} · ${formatCount(summary.tasks, 'task')}`}
      </div>
    </div>
  );
}

interface Props {
  role: SwarmRole;
  summary: RoleSummary;
  onNavigate: (targetId: string) => void;
}

/**
 * Detail panel body for the Managers or Workers group of the per-node view:
 * the managers' quorum (read live from the swarm), node states, capacity and
 * usage, engine versions, and the nodes, each linking to its own panel.
 */
export function DetailPanelRoleGroup({ role, summary, onNavigate }: Props) {
  const { theme } = useTheme();
  const mono = monoStyle(theme.panelText);
  const { cell, head } = panelTableStyles(theme.nodeSubtext);
  const monoCell: React.CSSProperties = { ...cell, fontFamily: 'var(--dg-font-mono)' };
  const { members, usage } = summary;
  const capacity = [
    summary.cores ? formatCores(summary.cores) : null,
    summary.memoryBytes ? formatBytes(summary.memoryBytes) : null,
  ].filter(Boolean).join(' · ');
  const versions = summary.engines.filter((e) => e.value !== 'unknown');

  // Asked once for the whole panel, so the quorum and the leader mark in the
  // table agree. The graph's leader flag freezes while quorum is lost, so it
  // only stands in until the swarm first answers.
  const managers = useSwarmManagers(role === 'manager');
  const isLeader = (name: string, graphLeader: boolean | undefined) => {
    if (managers.error) return false;
    if (managers.data) return managers.data.managers.some((m) => m.leader && m.name === name);
    return !!graphLeader;
  };
  const inGraph = new Set(members.map((m) => m.node.name));

  return (
    <>
      {role === 'manager' && (
        <QuorumSection managers={managers} inGraph={inGraph} onNavigate={onNavigate} />
      )}

      <Section title="Status">
        <Row label="State" value={tallyText(summary.states) || '—'} mono={mono} subtext={theme.nodeSubtext} />
        <Row label="Availability" value={tallyText(summary.availability) || '—'} mono={mono} subtext={theme.nodeSubtext} />
      </Section>

      {capacity && (
        <Section title="Resources">
          <Row label="Capacity" value={capacity} mono={mono} subtext={theme.nodeSubtext} />
          {usage ? (
            <>
              {usage.cpuPercent !== undefined && (
                <ResourceLine label="CPU" capacity={formatCores(usage.cores)} percent={usage.cpuPercent} color={cpuColor(usage.cpuPercent, 0)} />
              )}
              {usage.memPercent !== undefined && (
                <ResourceLine
                  label="Memory"
                  capacity={formatBytes(usage.memCapacity)}
                  used={formatBytes(usage.memUsed)}
                  percent={usage.memPercent}
                  color={theme.info}
                />
              )}
              {usage.reporting < members.length && (
                <Note>Usage of the {usage.reporting} of {members.length} nodes whose agent reports stats.</Note>
              )}
            </>
          ) : (
            <Note>No agent reports stats for these nodes.</Note>
          )}
        </Section>
      )}

      {summary.engines.length > 0 && (
        <Section title="Engine">
          {summary.engines.map((e) => (
            <Row key={e.value} label={e.value} value={formatCount(e.count, 'node')} mono={mono} subtext={theme.nodeSubtext} />
          ))}
          {versions.length > 1 && <Note>The nodes run different engine versions, as during a rolling upgrade.</Note>}
        </Section>
      )}

      <Section title={`Nodes (${members.length})`}>
        {members.length === 0 ? (
          <Note>No {role} nodes.</Note>
        ) : (
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 11, color: theme.panelText }}>
            <thead>
              <tr>
                <th style={head}>Node</th>
                <th style={head}>Tasks</th>
                <th style={head}>CPU</th>
                <th style={head}>Mem</th>
              </tr>
            </thead>
            <tbody>
              {members.map(({ node, tasks, usage: nodeUsage }) => {
                const state = node.swarmNode?.state ?? node.status ?? 'unknown';
                return (
                  <tr key={node.id} style={{ borderTop: `1px solid ${theme.panelBorder}` }}>
                    <td style={cell}>
                      <span style={{ display: 'inline-flex', alignItems: 'center', gap: 5 }}>
                        <span
                          title={`State: ${state}`}
                          style={{ width: 6, height: 6, borderRadius: '50%', background: swarmNodeStateColor(state), flexShrink: 0 }}
                        />
                        <NodeLink name={node.name} onNavigate={onNavigate} />
                        {isLeader(node.name, node.swarmNode?.leader) && (
                          <span title="Leader" style={{ color: swarmRoleColor('manager', theme.mode) }}>★</span>
                        )}
                      </span>
                    </td>
                    <td style={monoCell}>{tasks}</td>
                    <td style={monoCell}>{nodeUsage.cpuPercent !== undefined ? `${nodeUsage.cpuPercent.toFixed(0)}%` : '—'}</td>
                    <td style={monoCell}>{nodeUsage.memPercent !== undefined ? `${nodeUsage.memPercent.toFixed(0)}%` : '—'}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
      </Section>
    </>
  );
}

interface QuorumProps {
  managers: ReturnType<typeof useSwarmManagers>;
  /** Hostnames the graph has a node for; only those link to a panel. */
  inGraph: Set<string>;
  onNavigate: (targetId: string) => void;
}

/**
 * The managers' raft quorum, as the swarm answers it every few seconds. A
 * failed answer is shown as such, never the last known state: the swarm
 * stops answering exactly when quorum is lost.
 */
function QuorumSection({ managers: { data, error }, inGraph, onNavigate }: QuorumProps) {
  const { theme } = useTheme();
  const mono = monoStyle(theme.panelText);

  // A manager the graph doesn't show yet (just joined or promoted) would
  // open an empty panel, so it is named without a link.
  const managerName = (name: string) =>
    inGraph.has(name)
      ? <NodeLink name={name} onNavigate={onNavigate} />
      : <span style={{ fontFamily: 'var(--dg-font-mono)' }}>{name}</span>;

  if (error) {
    return (
      <Section title="Quorum">
        <Status label="unavailable" color={theme.danger} />
        <div style={{ fontSize: 11, color: theme.panelText }}>{error}</div>
      </Section>
    );
  }
  if (!data) {
    return (
      <Section title="Quorum">
        <Note>Asking the managers…</Note>
      </Section>
    );
  }

  const health = quorumHealth(data.managers);
  // Without a leader the swarm can't act, whatever the reachability.
  const status = health.spare < 0
    ? { label: 'quorum lost', color: theme.danger }
    : !health.leader
      ? { label: 'no leader', color: theme.danger }
      : health.unreachable.length > 0
        ? { label: 'degraded', color: theme.warning }
        : { label: 'healthy', color: theme.success };
  const canLose = health.spare > 0 ? formatCount(health.spare, 'more manager') : health.spare === 0 ? 'no more managers' : '—';

  return (
    <Section title="Quorum">
      <Status label={status.label} color={status.color} />
      <LinkRow label="Leader">{health.leader ? managerName(health.leader) : 'none'}</LinkRow>
      <Row label="Reachable" value={`${health.reachable} of ${formatCount(health.managers, 'manager')}`} mono={mono} subtext={theme.nodeSubtext} />
      <Row label="Majority" value={formatCount(health.quorum, 'manager')} mono={mono} subtext={theme.nodeSubtext} />
      <Row label="Can lose" value={canLose} mono={mono} subtext={theme.nodeSubtext} />
      {health.unreachable.length > 0 && (
        <LinkRow label="Unreachable">
          {/* Keyed by node ID: a rejoined manager leaves a stale entry under its hostname. */}
          {health.unreachable.map((m, i) => (
            <span key={m.id}>
              {i > 0 && ', '}
              {managerName(m.name)}
            </span>
          ))}
        </LinkRow>
      )}
      {health.managers % 2 === 0 && (
        <Note>
          An even number of managers adds no fault tolerance: {health.managers} tolerate as many failures as {health.managers - 1}.
        </Note>
      )}
    </Section>
  );
}

/** Link to a swarm node's own panel. */
function NodeLink({ name, onNavigate }: { name: string; onNavigate: (targetId: string) => void }) {
  const { theme } = useTheme();
  return (
    <button
      type="button"
      style={{ ...navButtonReset, fontFamily: 'var(--dg-font-mono)', ...navLinkStyle(theme.panelBorder) }}
      title={`Inspect node ${name}`}
      onClick={() => onNavigate(`swarmnode:${name}`)}
    >
      {name}
    </button>
  );
}

/** A label: value row whose value holds links rather than copyable text. */
function LinkRow({ label, children }: { label: string; children: ReactNode }) {
  const { theme } = useTheme();
  return (
    <div style={{ fontSize: 11, marginBottom: 3, color: theme.panelText }}>
      <span style={{ color: theme.nodeSubtext }}>{label}: </span>
      {children}
    </div>
  );
}

/** The quorum's status pill, above the section's rows. */
function Status({ label, color }: { label: string; color: string }) {
  return (
    <div style={{ marginBottom: 7 }}>
      <Pill label={label} color={color} />
    </div>
  );
}

function Note({ children }: { children: ReactNode }) {
  const { theme } = useTheme();
  return <div style={{ fontSize: 11, color: theme.nodeSubtext, fontStyle: 'italic', marginTop: 4 }}>{children}</div>;
}
