import { useTheme } from '../../theme';
import { formatBytes } from '../../utils/formatBytes';
import { STATUS_COLORS } from '../../utils/colors';
import { Section, Row, navLinkStyle, navButtonReset, monoStyle } from './shared';
import { Copyable } from './Copyable';
import { StatusBadge } from './StatusBadge';
import { DetailPanelStats } from './DetailPanelStats';
import { DetailPanelPorts } from './DetailPanelPorts';
import { DetailPanelMounts } from './DetailPanelMounts';
import { DetailPanelEnv } from './DetailPanelEnv';
import { DetailPanelLabels } from './DetailPanelLabels';
import { DetailPanelLogs } from './DetailPanelLogs';
import { taskContainerName } from '../../utils/stack';
import type { ContainerStatsData, ServiceDetail } from '../../types/stats';

/** Colour for each task state in the task table. */
const TASK_STATE_COLORS: Record<string, string> = {
  running: STATUS_COLORS.running,
  complete: STATUS_COLORS.created,
  failed: STATUS_COLORS.exited,
  rejected: STATUS_COLORS.exited,
  shutdown: STATUS_COLORS.not_running,
};

/** Header for a swarm service: name, image, status, mode and stack. */
export function DetailPanelServiceHeader({ detail }: { detail: ServiceDetail }) {
  const { theme } = useTheme();
  return (
    <div>
      <div style={{ fontSize: 16, fontWeight: 600, color: theme.nodeText, marginBottom: 2, wordBreak: 'break-all' }}>
        <Copyable value={detail.name}>{detail.name}</Copyable>
      </div>
      {detail.image && (
        <div style={{ fontSize: 11, color: theme.nodeSubtext, marginBottom: 6, wordBreak: 'break-all' }}>
          <Copyable value={detail.image}>{detail.image}</Copyable>
        </div>
      )}
      <div style={{ display: 'flex', alignItems: 'center', gap: 7, flexWrap: 'wrap' }}>
        <StatusBadge status={detail.status} />
        <span style={{ fontFamily: 'var(--dg-font-mono)', fontSize: 11, color: theme.nodeSubtext }}>
          service{detail.mode ? ` · ${detail.mode}` : ''}{detail.stack ? ` · stack ${detail.stack}` : ''}
        </span>
      </div>
    </div>
  );
}

interface Props {
  detail: ServiceDetail;
  /** Live stats map keyed by container or service name. */
  statsMap: Map<string, ContainerStatsData>;
  /** Whether the panel is visible (drives the log stream). */
  active: boolean;
  onNavigate: (targetId: string) => void;
}

/**
 * Detail panel body for a swarm service: replica/update state, the task table
 * (slot, node, state, error and per-task stats when the node agent reports
 * them), then the shared ports/mounts/env/labels sections and the service's
 * aggregated logs.
 */
export function DetailPanelService({ detail, statsMap, active, onNavigate }: Props) {
  const { theme } = useTheme();
  const mono = monoStyle(theme.panelText);
  const tasks = detail.tasks ?? [];
  const ports = (detail.ports ?? []).map((p) => ({
    hostPort: p.host ? String(p.host) : '—',
    containerPort: String(p.container),
    protocol: p.publishMode ? `${p.protocol} (${p.publishMode})` : p.protocol,
  }));

  const cell: React.CSSProperties = { padding: '3px 6px 3px 0', verticalAlign: 'top' };
  const head: React.CSSProperties = {
    ...cell,
    fontFamily: 'var(--dg-font-mono)',
    fontSize: 9,
    fontWeight: 600,
    color: theme.nodeSubtext,
    textTransform: 'uppercase',
    letterSpacing: '0.06em',
    textAlign: 'left',
  };

  return (
    <>
      <DetailPanelStats stats={statsMap.get(detail.name)} />

      <Section title="Replicas">
        <Row
          label="Running"
          value={detail.replicas ? `${detail.replicas.running} / ${detail.replicas.desired}` : 'Unknown'}
          mono={mono}
          subtext={theme.nodeSubtext}
        />
        {detail.mode && <Row label="Mode" value={detail.mode} mono={mono} subtext={theme.nodeSubtext} />}
        {detail.updateStatus?.state && (
          <Row
            label="Update"
            value={detail.updateStatus.message ? `${detail.updateStatus.state} — ${detail.updateStatus.message}` : detail.updateStatus.state}
            mono={mono}
            subtext={theme.nodeSubtext}
          />
        )}
        {detail.constraints && detail.constraints.length > 0 && (
          <Row label="Constraints" value={detail.constraints.join(', ')} mono={mono} subtext={theme.nodeSubtext} />
        )}
      </Section>

      <Section title={detail.tasksUnavailable ? 'Tasks' : `Tasks (${tasks.length})`}>
        {detail.tasksUnavailable ? (
          <div style={{ fontSize: 11, color: theme.nodeSubtext }}>Tasks unavailable.</div>
        ) : tasks.length === 0 ? (
          <div style={{ fontSize: 11, color: theme.nodeSubtext }}>No tasks.</div>
        ) : (
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 11, color: theme.panelText }}>
            <thead>
              <tr>
                <th style={head}>Slot</th>
                <th style={head}>Node</th>
                <th style={head}>State</th>
                <th style={head}>Stats</th>
              </tr>
            </thead>
            <tbody>
              {tasks.map((t) => {
                const s = statsMap.get(taskContainerName(detail.name, t));
                const color = TASK_STATE_COLORS[t.state ?? ''] ?? STATUS_COLORS.unhealthy;
                return (
                  <tr key={t.id} style={{ borderTop: `1px solid ${theme.panelBorder}` }}>
                    <td style={{ ...cell, fontFamily: 'var(--dg-font-mono)' }}>{t.slot || '—'}</td>
                    <td style={{ ...cell, fontFamily: 'var(--dg-font-mono)', wordBreak: 'break-all' }} title={t.nodeId}>
                      {t.nodeHostname || t.nodeId?.slice(0, 12) || '—'}
                    </td>
                    <td style={cell}>
                      <span style={{ display: 'inline-flex', alignItems: 'center', gap: 5 }}>
                        <span style={{ width: 6, height: 6, borderRadius: '50%', background: color, flexShrink: 0 }} />
                        {t.state ?? 'unknown'}
                      </span>
                      {t.error && (
                        <div style={{ color: theme.danger, fontSize: 10.5, marginTop: 2, wordBreak: 'break-word' }} title={t.timestamp}>
                          {t.error}
                        </div>
                      )}
                    </td>
                    <td style={{ ...cell, fontFamily: 'var(--dg-font-mono)', fontSize: 10.5, color: theme.nodeSubtext, whiteSpace: 'nowrap' }}>
                      {s ? `${s.cpuPercent.toFixed(1)}% · ${formatBytes(s.memUsage)}` : '—'}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
      </Section>

      <DetailPanelPorts ports={ports} />
      <DetailPanelMounts mounts={detail.mounts ?? []} onNavigate={onNavigate} />

      {detail.networks && detail.networks.length > 0 && (
        <Section title="Networks">
          {detail.networks.map((n) => (
            <div key={n.name} style={{ fontSize: 11, color: theme.panelText, marginBottom: 3 }}>
              <button
                type="button"
                style={{ ...navButtonReset, fontFamily: 'var(--dg-font-mono)', ...navLinkStyle(theme.panelBorder) }}
                title={`Inspect network ${n.name}`}
                onClick={() => onNavigate(`network:${n.name}`)}
              >
                {n.name}
              </button>
              {n.aliases && n.aliases.length > 0 && (
                <span style={{ color: theme.nodeSubtext }}> ({n.aliases.join(', ')})</span>
              )}
            </div>
          ))}
        </Section>
      )}

      <DetailPanelEnv env={detail.env ?? []} />
      <DetailPanelLabels labels={detail.labels ?? undefined} />
      <DetailPanelLogs containerId={detail.name} active={active} resource="service" />
    </>
  );
}
