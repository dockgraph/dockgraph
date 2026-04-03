import { useTheme } from '../../theme';
import { Section } from './shared';
import { STATUS_COLORS } from '../../utils/colors';
import type { ContainerDetail } from '../../types/stats';

interface Props {
  health: ContainerDetail['health'] | undefined;
}

export function DetailPanelHealth({ health }: Props) {
  const { theme } = useTheme();
  if (!health) return null;

  const color = health.status === 'healthy' ? STATUS_COLORS.running : health.status === 'unhealthy' ? STATUS_COLORS.unhealthy : theme.nodeSubtext;

  return (
    <Section title="Health Check">
      <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 6 }}>
        <span style={{ width: 8, height: 8, borderRadius: '50%', background: color, flexShrink: 0 }} />
        <span style={{ fontSize: 12, color: theme.panelText, textTransform: 'capitalize' }}>{health.status}</span>
        {health.failingStreak > 0 && <span style={{ fontSize: 10, color: theme.warning }}>({health.failingStreak} failing)</span>}
      </div>
      {health.log?.slice(-5).map((entry, i) => (
        <div key={i} style={{ fontSize: 10, color: entry.exitCode === 0 ? theme.nodeSubtext : theme.danger, marginBottom: 2, fontFamily: 'monospace' }}>
          exit {entry.exitCode}{entry.output ? `: ${entry.output.slice(0, 80)}` : ''}
        </div>
      ))}
    </Section>
  );
}
