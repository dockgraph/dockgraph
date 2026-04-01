import { useTheme } from '../../theme';
import { Section } from './shared';
import type { ContainerDetail } from '../../types/stats';

interface Props {
  ports: ContainerDetail['ports'];
}

export function DetailPanelPorts({ ports }: Props) {
  const { theme } = useTheme();
  if (!ports?.length) return null;

  return (
    <Section title="Ports">
      {ports.map((p, i) => (
        <div key={i} style={{ fontSize: 11, color: theme.panelText, marginBottom: 2 }}>
          <span style={{ fontFamily: 'monospace' }}>{p.hostPort}</span>
          <span style={{ color: theme.nodeSubtext }}> → </span>
          <span style={{ fontFamily: 'monospace' }}>{p.containerPort}/{p.protocol}</span>
        </div>
      ))}
    </Section>
  );
}
