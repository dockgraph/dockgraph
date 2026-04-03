import { useTheme } from '../../theme';
import { Section, Row } from './shared';
import { monoStyle } from './panelStyles';
import { KeyValueList } from './KeyValueList';
import { ContainerLink } from './ContainerLink';
import { formatBytes } from '../../utils/formatBytes';
import type { VolumeDetail } from '../../types/stats';
import type { VolumeMount } from '../../types';

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
              <ContainerLink name={node.name} onNavigate={onNavigate} />
              {mountPath && <Row label="Mount" value={mountPath} mono={mono} subtext={theme.nodeSubtext} />}
            </div>
          ))}
        </Section>
      )}

      {volume.options && Object.keys(volume.options).length > 0 && (
        <Section title="Options">
          <KeyValueList entries={volume.options} />
        </Section>
      )}

      {volume.labels && Object.keys(volume.labels).length > 0 && (
        <Section title="Labels">
          <KeyValueList entries={volume.labels} />
        </Section>
      )}

      {volume.status && Object.keys(volume.status).length > 0 && (
        <Section title="Status">
          <KeyValueList entries={volume.status} />
        </Section>
      )}
    </>
  );
}
