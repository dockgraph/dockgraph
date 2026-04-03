import { useTheme } from '../../theme';
import { formatBytes } from '../../utils/formatBytes';
import { cpuColor } from '../../utils/colors';
import { Section } from './shared';
import type { ContainerStatsData } from '../../types/stats';

interface Props {
  stats: ContainerStatsData | undefined;
}

function StatBar({ label, value, max, color }: { label: string; value: number; max: number; color: string }) {
  const { theme } = useTheme();
  const pct = max > 0 ? Math.min((value / max) * 100, 100) : 0;
  return (
    <div style={{ marginBottom: 6 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 11, color: theme.panelText, marginBottom: 2 }}>
        <span>{label}</span>
        <span>{pct.toFixed(1)}%</span>
      </div>
      <div style={{ height: 4, background: theme.portBg, borderRadius: 2 }}>
        <div style={{ width: `${pct}%`, height: '100%', background: color, borderRadius: 2 }} />
      </div>
    </div>
  );
}

export function DetailPanelStats({ stats }: Props) {
  const { theme } = useTheme();
  if (!stats) return <div style={{ fontSize: 11, color: theme.nodeSubtext, padding: '8px 0' }}>No stats available</div>;

  const color = cpuColor(stats.cpuPercent, stats.cpuThrottled);

  return (
    <Section title="Resources">
      <StatBar
        label={stats.cpuThrottled > 0 ? `CPU (throttled ${stats.cpuThrottled.toFixed(1)}%)` : 'CPU'}
        value={stats.cpuPercent}
        max={100}
        color={color}
      />
      {stats.memUsage > 0 ? (
        <>
          <StatBar label="Memory" value={stats.memUsage} max={stats.memLimit || stats.memUsage} color="#3b82f6" />
          <div style={{ fontSize: 10, color: theme.nodeSubtext, marginBottom: 8 }}>
            {formatBytes(stats.memUsage)} / {stats.memLimit ? formatBytes(stats.memLimit) : '∞'}
          </div>
        </>
      ) : (
        <div style={{ fontSize: 10, color: theme.nodeSubtext, marginBottom: 8 }}>Memory: N/A (cgroup not available)</div>
      )}
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 4, fontSize: 10, color: theme.panelText }}>
        <span>Net Rx: {formatBytes(stats.netRx)}</span>
        <span>Net Tx: {formatBytes(stats.netTx)}</span>
        <span>Disk Read: {formatBytes(stats.blockRead)}</span>
        <span>Disk Write: {formatBytes(stats.blockWrite)}</span>
        {stats.pids > 0 && <span>PIDs: {stats.pids}</span>}
        {(stats.netRxErrors > 0 || stats.netTxErrors > 0) && (
          <span style={{ color: '#f59e0b' }}>Errors: {stats.netRxErrors + stats.netTxErrors}</span>
        )}
      </div>
    </Section>
  );
}

