import { useTheme } from '../../theme';
import { Section, Row, navLinkStyle } from './shared';
import type { ContainerDetail } from '../../types/stats';

interface Props {
  networkMode: string;
  networks: ContainerDetail['networks'];
  onNavigate?: (nodeId: string) => void;
}

export function DetailPanelNetwork({ networkMode, networks, onNavigate }: Props) {
  const { theme } = useTheme();
  const mono: React.CSSProperties = { fontFamily: 'monospace', fontSize: 11, color: theme.panelText, wordBreak: 'break-all' };

  return (
    <Section title="Networking">
      <Row label="Mode" value={networkMode} mono={mono} subtext={theme.nodeSubtext} />
      {networks?.map((n) => (
        <div key={n.name} style={{ marginBottom: 6 }}>
          <div
            style={{
              fontSize: 11,
              fontWeight: 600,
              color: theme.panelText,
              marginBottom: 4,
              ...(onNavigate ? navLinkStyle(theme.panelBorder) : {}),
            }}
            onClick={onNavigate ? () => onNavigate(`network:${n.name}`) : undefined}
            title={onNavigate ? `Inspect ${n.name}` : undefined}
          >
            {n.name}
          </div>
          <Row label="IP" value={n.ipAddress || '—'} mono={mono} subtext={theme.nodeSubtext} />
          <Row label="Gateway" value={n.gateway || '—'} mono={mono} subtext={theme.nodeSubtext} />
          {n.macAddress && <Row label="MAC" value={n.macAddress} mono={mono} subtext={theme.nodeSubtext} />}
        </div>
      ))}
    </Section>
  );
}
