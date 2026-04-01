import { useTheme } from '../../theme';
import { Section } from './DetailPanelStats';
import type { ContainerDetail } from '../../types/stats';

interface Props {
  mounts: ContainerDetail['mounts'];
}

export function DetailPanelMounts({ mounts }: Props) {
  const { theme } = useTheme();
  if (!mounts?.length) return null;

  return (
    <Section title="Mounts">
      {mounts.map((m, i) => (
        <div key={i} style={{ fontSize: 11, color: theme.panelText, marginBottom: 4 }}>
          <div style={{ fontFamily: 'monospace', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={m.source}>
            {m.source}
          </div>
          <div style={{ fontSize: 10, color: theme.nodeSubtext }}>
            → {m.destination} ({m.type}{m.rw ? '' : ', ro'})
          </div>
        </div>
      ))}
    </Section>
  );
}
