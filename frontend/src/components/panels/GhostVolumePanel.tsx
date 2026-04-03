import { useTheme } from '../../theme';
import { Section } from './shared';
import { monoStyle } from './panelStyles';
import { ContainerLink } from './ContainerLink';
import type { DGNode, VolumeMount } from '../../types';

interface Props {
  node: DGNode;
  mounts: VolumeMount[];
  onNavigate: (targetId: string) => void;
}

export function GhostVolumePanel({ node, mounts, onNavigate }: Props) {
  const { theme } = useTheme();
  const mono = monoStyle(theme.panelText);

  return (
    <>
      {mounts.length > 0 && (
        <Section title={`Containers (${mounts.length})`}>
          {mounts.map(({ node: container, mountPath }) => (
            <div key={container.id} style={{ marginBottom: 6 }}>
              <ContainerLink name={container.name} onNavigate={onNavigate} />
              {mountPath && (
                <div style={{ fontSize: 11, marginBottom: 3 }}>
                  <span style={{ color: theme.nodeSubtext }}>Mount: </span>
                  <span style={mono}>{mountPath}</span>
                </div>
              )}
            </div>
          ))}
        </Section>
      )}
      {node.source && (
        <div style={{ fontSize: 11, color: theme.nodeSubtext }}>
          Defined in {node.source}
        </div>
      )}
    </>
  );
}
