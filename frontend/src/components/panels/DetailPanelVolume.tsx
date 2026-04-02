import { useTheme } from '../../theme';
import { Section, Row } from './shared';
import { monoStyle, navLinkStyle } from './panelStyles';
import { formatBytes } from '../../utils/formatBytes';
import type { VolumeDetail } from '../../types/stats';
import type { DGNode } from '../../types';

interface VolumeMount {
  node: DGNode;
  mountPath: string;
}

interface Props {
  volume: VolumeDetail;
  mounts?: VolumeMount[];
  onNavigate?: (targetId: string) => void;
}

export function DetailPanelVolume({ volume, mounts, onNavigate }: Props) {
  const { theme } = useTheme();
  const mono = monoStyle(theme.panelText);

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

      {mounts && mounts.length > 0 && (
        <Section title={`Containers (${mounts.length})`}>
          {mounts.map(({ node, mountPath }) => (
            <div key={node.id} style={{ marginBottom: 6 }}>
              <div
                style={{
                  fontSize: 11,
                  fontWeight: 600,
                  color: theme.panelText,
                  marginBottom: 4,
                  ...(onNavigate ? navLinkStyle(theme.panelBorder) : {}),
                }}
                onClick={onNavigate ? () => onNavigate(`container:${node.name}`) : undefined}
                title={onNavigate ? `Inspect ${node.name}` : undefined}
              >
                {node.name}
              </div>
              {mountPath && <Row label="Mount" value={mountPath} mono={mono} subtext={theme.nodeSubtext} />}
            </div>
          ))}
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
