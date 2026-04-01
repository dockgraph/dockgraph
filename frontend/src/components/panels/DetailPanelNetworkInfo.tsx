import { useTheme } from '../../theme';
import { Section, Row } from './DetailPanelStats';
import type { NetworkDetail } from '../../types/stats';

interface Props {
  network: NetworkDetail;
}

export function DetailPanelNetworkInfo({ network }: Props) {
  const { theme } = useTheme();
  const mono: React.CSSProperties = { fontFamily: 'monospace', fontSize: 11, color: theme.panelText, wordBreak: 'break-all' };

  return (
    <>
      <Section title="General">
        <Row label="Driver" value={network.driver} mono={mono} subtext={theme.nodeSubtext} />
        <Row label="Scope" value={network.scope} mono={mono} subtext={theme.nodeSubtext} />
        <Row label="ID" value={network.id.slice(0, 12)} mono={mono} subtext={theme.nodeSubtext} />
        {network.created && <Row label="Created" value={network.created} mono={mono} subtext={theme.nodeSubtext} />}
        <Row label="Internal" value={network.internal ? 'yes' : 'no'} mono={mono} subtext={theme.nodeSubtext} />
        <Row label="IPv6" value={network.enableIPv6 ? 'yes' : 'no'} mono={mono} subtext={theme.nodeSubtext} />
      </Section>

      {network.ipam?.config && network.ipam.config.length > 0 && (
        <Section title="IPAM">
          <Row label="Driver" value={network.ipam.driver} mono={mono} subtext={theme.nodeSubtext} />
          {network.ipam.config.map((cfg, i) => (
            <div key={i} style={{ marginBottom: 6 }}>
              {cfg.subnet && <Row label="Subnet" value={cfg.subnet} mono={mono} subtext={theme.nodeSubtext} />}
              {cfg.gateway && <Row label="Gateway" value={cfg.gateway} mono={mono} subtext={theme.nodeSubtext} />}
              {cfg.ipRange && <Row label="IP Range" value={cfg.ipRange} mono={mono} subtext={theme.nodeSubtext} />}
            </div>
          ))}
        </Section>
      )}

      {network.containers && network.containers.length > 0 && (
        <Section title="Connected Containers">
          {network.containers.map((c) => (
            <div key={c.name} style={{ marginBottom: 6 }}>
              <div style={{ fontSize: 11, fontWeight: 600, color: theme.panelText, marginBottom: 2 }}>{c.name}</div>
              {c.ipv4Address && <Row label="IPv4" value={c.ipv4Address} mono={mono} subtext={theme.nodeSubtext} />}
              {c.macAddress && <Row label="MAC" value={c.macAddress} mono={mono} subtext={theme.nodeSubtext} />}
            </div>
          ))}
        </Section>
      )}

      {network.options && Object.keys(network.options).length > 0 && (
        <Section title="Options">
          {Object.entries(network.options).map(([k, v]) => (
            <div key={k} style={{ fontSize: 11, marginBottom: 2 }}>
              <span style={{ color: theme.nodeSubtext }}>{k}=</span>
              <span style={mono}>{v}</span>
            </div>
          ))}
        </Section>
      )}

      {network.labels && Object.keys(network.labels).length > 0 && (
        <Section title="Labels">
          {Object.entries(network.labels).map(([k, v]) => (
            <div key={k} style={{ fontSize: 11, marginBottom: 2 }}>
              <span style={{ color: theme.nodeSubtext }}>{k}=</span>
              <span style={mono}>{v}</span>
            </div>
          ))}
        </Section>
      )}
    </>
  );
}
