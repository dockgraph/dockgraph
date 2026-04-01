import { useTheme } from '../../theme';
import { Section, Row } from './DetailPanelStats';
import { formatBytes } from '../../utils/formatBytes';
import type { VolumeDetail } from '../../types/stats';

interface Props {
  volume: VolumeDetail;
}

export function DetailPanelVolume({ volume }: Props) {
  const { theme } = useTheme();
  const mono: React.CSSProperties = { fontFamily: 'monospace', fontSize: 11, color: theme.panelText, wordBreak: 'break-all' };

  return (
    <>
      <Section title="General">
        <Row label="Driver" value={volume.driver} mono={mono} subtext={theme.nodeSubtext} />
        <Row label="Scope" value={volume.scope} mono={mono} subtext={theme.nodeSubtext} />
        <div style={{ marginBottom: 4 }}>
          <span style={{ fontSize: 10, color: theme.nodeSubtext }}>Mountpoint: </span>
          <span style={mono}>{volume.mountpoint}</span>
        </div>
        {volume.createdAt && <Row label="Created" value={volume.createdAt} mono={mono} subtext={theme.nodeSubtext} />}
      </Section>

      {(volume.usageSize > 0 || volume.usageRefCount > 0) && (
        <Section title="Usage">
          {volume.usageSize > 0 && <Row label="Size" value={formatBytes(volume.usageSize)} mono={mono} subtext={theme.nodeSubtext} />}
          {volume.usageRefCount > 0 && <Row label="Reference Count" value={String(volume.usageRefCount)} mono={mono} subtext={theme.nodeSubtext} />}
        </Section>
      )}

      {volume.options && Object.keys(volume.options).length > 0 && (
        <Section title="Options">
          {Object.entries(volume.options).map(([k, v]) => (
            <div key={k} style={{ fontSize: 11, marginBottom: 2 }}>
              <span style={{ color: theme.nodeSubtext }}>{k}=</span>
              <span style={mono}>{v}</span>
            </div>
          ))}
        </Section>
      )}

      {volume.labels && Object.keys(volume.labels).length > 0 && (
        <Section title="Labels">
          {Object.entries(volume.labels).map(([k, v]) => (
            <div key={k} style={{ fontSize: 11, marginBottom: 2 }}>
              <span style={{ color: theme.nodeSubtext }}>{k}=</span>
              <span style={mono}>{v}</span>
            </div>
          ))}
        </Section>
      )}

      {volume.status && Object.keys(volume.status).length > 0 && (
        <Section title="Status">
          {Object.entries(volume.status).map(([k, v]) => (
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

