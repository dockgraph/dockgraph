import { useTheme } from '../../theme';
import { formatBytes } from '../../utils/formatBytes';
import { cpuColor } from '../../utils/colors';
import { Section } from './shared';
import type { ContainerStatsData } from '../../types/stats';

interface Props {
  stats: ContainerStatsData | undefined;
}

function StatCard({ label, value, unit, sub, barPct, color }: { label: string; value: string; unit?: string; sub?: string; barPct: number | null; color: string }) {
  const { theme } = useTheme();
  return (
    <div
      style={{
        background: theme.portBg,
        border: `1px solid ${theme.panelBorder}`,
        borderRadius: 8,
        padding: '9px 11px',
      }}
    >
      <div style={{ fontFamily: 'var(--dg-font-mono)', fontSize: 9, fontWeight: 600, letterSpacing: '0.06em', textTransform: 'uppercase', color: theme.nodeSubtext }}>
        {label}
      </div>
      <div style={{ fontFamily: 'var(--dg-font-mono)', fontSize: 17, fontWeight: 600, color: theme.nodeText, marginTop: 2, lineHeight: 1.1 }}>
        {value}
        {unit && <span style={{ fontSize: 11, fontWeight: 500, color: theme.nodeSubtext }}> {unit}</span>}
      </div>
      {sub && <div style={{ fontFamily: 'var(--dg-font-mono)', fontSize: 10, color: theme.nodeSubtext, marginTop: 1 }}>{sub}</div>}
      {barPct !== null && (
        <div style={{ height: 4, background: theme.canvasBg, borderRadius: 2, marginTop: 7, overflow: 'hidden' }}>
          <div style={{ width: `${Math.min(barPct, 100)}%`, height: '100%', background: color, borderRadius: 2 }} />
        </div>
      )}
    </div>
  );
}

function MiniStat({ label, value, color }: { label: string; value: string; color?: string }) {
  const { theme } = useTheme();
  return (
    <div style={{ display: 'flex', justifyContent: 'space-between', gap: 8, fontSize: 10.5 }}>
      <span style={{ color: theme.nodeSubtext }}>{label}</span>
      <span style={{ fontFamily: 'var(--dg-font-mono)', color: color ?? theme.panelText }}>{value}</span>
    </div>
  );
}

export function DetailPanelStats({ stats }: Props) {
  const { theme } = useTheme();
  if (!stats) return <div style={{ fontSize: 11, color: theme.nodeSubtext, padding: '8px 0' }}>No stats available</div>;

  const cpu = cpuColor(stats.cpuPercent, stats.cpuThrottled);
  const memPct = stats.memUsage > 0 ? (stats.memUsage / (stats.memLimit || stats.memUsage)) * 100 : null;
  const errors = stats.netRxErrors + stats.netTxErrors;

  return (
    <Section title="Resources">
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8, marginBottom: 10 }}>
        <StatCard
          label={stats.cpuThrottled > 0 ? `CPU · thr ${stats.cpuThrottled.toFixed(0)}%` : 'CPU'}
          value={stats.cpuPercent.toFixed(1)}
          unit="%"
          barPct={stats.cpuPercent}
          color={cpu}
        />
        {stats.memUsage > 0 ? (
          <StatCard
            label="Memory"
            value={formatBytes(stats.memUsage)}
            sub={`/ ${stats.memLimit ? formatBytes(stats.memLimit) : '∞'}`}
            barPct={memPct}
            color={theme.info}
          />
        ) : (
          <StatCard label="Memory" value="N/A" sub="cgroup n/a" barPct={null} color={theme.info} />
        )}
      </div>
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', columnGap: 16, rowGap: 4 }}>
        <MiniStat label="Net Rx" value={formatBytes(stats.netRx)} />
        <MiniStat label="Net Tx" value={formatBytes(stats.netTx)} />
        <MiniStat label="Disk Read" value={formatBytes(stats.blockRead)} />
        <MiniStat label="Disk Write" value={formatBytes(stats.blockWrite)} />
        {stats.pids > 0 && <MiniStat label="PIDs" value={String(stats.pids)} />}
        {errors > 0 && <MiniStat label="Errors" value={String(errors)} color={theme.warning} />}
      </div>
    </Section>
  );
}

