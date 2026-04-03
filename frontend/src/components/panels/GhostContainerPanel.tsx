import { useTheme } from '../../theme';
import { Section } from './shared';
import { DetailPanelCompose } from './DetailPanelCompose';
import type { DGNode } from '../../types';

interface Props {
  node: DGNode;
  onNavigate: (targetId: string) => void;
}

export function GhostContainerPanel({ node, onNavigate }: Props) {
  const { theme } = useTheme();

  return (
    <>
      {node.compose && (
        <DetailPanelCompose
          compose={node.compose}
          image={node.image}
          onNavigate={onNavigate}
        />
      )}
      {node.ports && node.ports.length > 0 && (
        <Section title="Ports">
          {node.ports.map((p, i) => (
            <div key={i} style={{ fontSize: 11, color: theme.panelText, marginBottom: 2 }}>
              <span style={{ fontFamily: 'monospace' }}>{p.host}</span>
              <span style={{ color: theme.nodeSubtext }}> → </span>
              <span style={{ fontFamily: 'monospace' }}>{p.container}</span>
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
