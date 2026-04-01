import { useTheme } from '../../theme';
import { Section } from './DetailPanelStats';
import type { ContainerDetail } from '../../types/stats';

interface Props {
  networkMode: string;
  networks: ContainerDetail['networks'];
}

export function DetailPanelNetwork({ networkMode, networks }: Props) {
  const { theme } = useTheme();

  return (
    <Section title="Networking">
      <div style={{ fontSize: 11, color: theme.panelText, marginBottom: 6 }}>
        Mode: <span style={{ fontFamily: 'monospace' }}>{networkMode}</span>
      </div>
      {networks?.map((n) => (
        <div key={n.name} style={{ fontSize: 11, marginBottom: 4 }}>
          <div style={{ fontWeight: 600, color: theme.panelText }}>{n.name}</div>
          <div style={{ color: theme.nodeSubtext, fontFamily: 'monospace', fontSize: 10 }}>
            IP: {n.ipAddress || '\u2014'} · GW: {n.gateway || '\u2014'}
          </div>
          {n.macAddress && <div style={{ color: theme.nodeSubtext, fontFamily: 'monospace', fontSize: 10 }}>MAC: {n.macAddress}</div>}
        </div>
      ))}
    </Section>
  );
}
