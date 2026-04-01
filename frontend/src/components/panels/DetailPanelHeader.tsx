import { useTheme } from '../../theme';
import { STATUS_COLORS, STATUS_LABELS } from '../../utils/colors';
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
      <div style={{ fontSize: 16, fontWeight: 600, color: theme.nodeText, marginBottom: 2, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={detail.name}>
        {detail.name}
      </div>
      <div style={{ fontSize: 11, color: theme.nodeSubtext, marginBottom: 6, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={detail.image}>
        {detail.image}
      </div>
      <div style={{ display: 'flex', alignItems: 'center', gap: 6, flexWrap: 'wrap' }}>
        <span style={{ width: 8, height: 8, borderRadius: '50%', background: statusColor, display: 'inline-block', flexShrink: 0 }} />
        <span style={{ fontSize: 12, color: theme.panelText }}>{STATUS_LABELS[detail.status] ?? detail.status}</span>
        {uptime && <span style={{ fontSize: 11, color: theme.nodeSubtext }}>{detail.running ? `Up ${uptime}` : `Exited ${uptime} ago`}</span>}
        {detail.oomKilled && <span style={{ fontSize: 10, color: '#ef4444', fontWeight: 600 }}>OOM Killed</span>}
        {!detail.running && detail.exitCode !== 0 && <span style={{ fontSize: 10, color: theme.nodeSubtext }}>code {detail.exitCode}</span>}
      </div>
    </div>
  );
}
