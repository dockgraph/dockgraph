import { useTheme } from '../../theme';
import { STATUS_COLORS, STATUS_LABELS } from '../../utils/colors';
import { Copyable } from './Copyable';
import type { ContainerDetail } from '../../types/stats';

interface Props {
  detail: ContainerDetail;
}

function formatUptime(startedAt: string, finishedAt: string, running: boolean): string {
  const ref = running ? startedAt : finishedAt;
  if (!ref) return '';
  const ms = Date.now() - new Date(ref).getTime();
  if (ms < 0) return '';
  const sec = Math.floor(ms / 1000);
  if (sec < 60) return `${sec}s`;
  const min = Math.floor(sec / 60);
  if (min < 60) return `${min}m`;
  const hr = Math.floor(min / 60);
  if (hr < 24) return `${hr}h ${min % 60}m`;
  const days = Math.floor(hr / 24);
  return `${days}d ${hr % 24}h`;
}

export function DetailPanelHeader({ detail }: Props) {
  const { theme } = useTheme();
  const statusColor = STATUS_COLORS[detail.status] ?? STATUS_COLORS.exited;
  const uptime = formatUptime(detail.startedAt, detail.finishedAt, detail.running);

  return (
    <div>
      <div style={{ fontFamily: 'var(--dg-font-mono)', fontSize: 15, fontWeight: 600, letterSpacing: '-0.01em', color: theme.nodeText, marginBottom: 3, wordBreak: 'break-all' }}>
        <Copyable value={detail.name}>{detail.name}</Copyable>
      </div>
      <div style={{ fontFamily: 'var(--dg-font-mono)', fontSize: 11, color: theme.nodeSubtext, marginBottom: 8, wordBreak: 'break-all' }}>
        <Copyable value={detail.image}>{detail.image}</Copyable>
      </div>
      <div style={{ display: 'flex', alignItems: 'center', gap: 7, flexWrap: 'wrap' }}>
        <span
          style={{
            display: 'inline-flex',
            alignItems: 'center',
            gap: 6,
            padding: '3px 9px',
            borderRadius: 999,
            background: `${statusColor}22`,
            fontFamily: 'var(--dg-font-mono)',
            fontSize: 10,
            fontWeight: 600,
            letterSpacing: '0.04em',
            textTransform: 'uppercase',
            color: statusColor,
          }}
        >
          <span style={{ width: 6, height: 6, borderRadius: '50%', background: statusColor, flexShrink: 0, boxShadow: detail.running ? `0 0 6px ${statusColor}` : 'none' }} />
          {STATUS_LABELS[detail.status] ?? detail.status}
        </span>
        {uptime && <span style={{ fontFamily: 'var(--dg-font-mono)', fontSize: 11, color: theme.nodeSubtext }}>{detail.running ? `up ${uptime}` : `exited ${uptime} ago`}</span>}
        {detail.oomKilled && <span style={{ fontFamily: 'var(--dg-font-mono)', fontSize: 10, color: theme.danger, fontWeight: 600 }}>OOM KILLED</span>}
        {!detail.running && detail.exitCode !== 0 && <span style={{ fontFamily: 'var(--dg-font-mono)', fontSize: 10, color: theme.nodeSubtext }}>code {detail.exitCode}</span>}
      </div>
    </div>
  );
}
