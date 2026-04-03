import { useTheme } from '../../theme';
import { Section, Row, monoStyle } from './shared';
import type { ContainerDetail } from '../../types/stats';

interface Props {
  detail: ContainerDetail;
}

export function DetailPanelProcess({ detail }: Props) {
  const { theme } = useTheme();
  const mono = monoStyle(theme.panelText);

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
