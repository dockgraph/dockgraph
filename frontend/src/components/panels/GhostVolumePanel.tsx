import { useTheme } from '../../theme';
import { Section } from './shared';
import { monoStyle, navLinkStyle } from './panelStyles';
import type { DGNode } from '../../types';

interface VolumeMount {
  node: DGNode;
  mountPath: string;
}

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
              <div
                style={{
                  fontSize: 11,
                  fontWeight: 600,
                  color: theme.panelText,
                  marginBottom: 4,
                  ...navLinkStyle(theme.panelBorder),
                }}
                onClick={() => onNavigate(`container:${container.name}`)}
                title={`Inspect ${container.name}`}
              >
                {container.name}
              </div>
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
