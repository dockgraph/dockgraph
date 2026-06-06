import { useTheme } from '../../theme';
import { Section } from './shared';
import { Copyable } from './Copyable';
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
        <Copyable
          key={i}
          value={`${p.hostPort}:${p.containerPort}/${p.protocol}`}
          style={{ display: 'block', fontSize: 11, color: theme.panelText, marginBottom: 2 }}
        >
          <span style={{ fontFamily: 'var(--dg-font-mono)' }}>{p.hostPort}</span>
          <span style={{ color: theme.nodeSubtext }}> → </span>
          <span style={{ fontFamily: 'var(--dg-font-mono)' }}>{p.containerPort}/{p.protocol}</span>
        </Copyable>
      ))}
    </Section>
  );
}
