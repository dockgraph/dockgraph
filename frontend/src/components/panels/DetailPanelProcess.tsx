import { useTheme } from '../../theme';
import { Section } from './DetailPanelStats';
import type { ContainerDetail } from '../../types/stats';

interface Props {
  detail: ContainerDetail;
}

export function DetailPanelProcess({ detail }: Props) {
  const { theme } = useTheme();
  const mono: React.CSSProperties = { fontFamily: 'monospace', fontSize: 11, color: theme.panelText, wordBreak: 'break-all' };

  return (
    <Section title="Process">
      {detail.cmd?.length > 0 && <Row label="Command" value={detail.cmd.join(' ')} mono={mono} subtext={theme.nodeSubtext} />}
      {detail.entrypoint?.length > 0 && <Row label="Entrypoint" value={detail.entrypoint.join(' ')} mono={mono} subtext={theme.nodeSubtext} />}
      {detail.workingDir && <Row label="Working Dir" value={detail.workingDir} mono={mono} subtext={theme.nodeSubtext} />}
      {detail.user && <Row label="User" value={detail.user} mono={mono} subtext={theme.nodeSubtext} />}
      {detail.pid > 0 && <Row label="PID" value={String(detail.pid)} mono={mono} subtext={theme.nodeSubtext} />}
      <Row label="Restart" value={`${detail.restartPolicy.Name}${detail.restartPolicy.MaximumRetryCount ? ` (max ${detail.restartPolicy.MaximumRetryCount})` : ''}`} mono={mono} subtext={theme.nodeSubtext} />
    </Section>
  );
}

function Row({ label, value, mono, subtext }: { label: string; value: string; mono: React.CSSProperties; subtext: string }) {
  return (
    <div style={{ marginBottom: 4 }}>
      <span style={{ fontSize: 10, color: subtext }}>{label}: </span>
      <span style={mono}>{value}</span>
    </div>
  );
}
