import { useTheme } from '../../theme';
import { Section, Row } from './shared';
import { monoStyle } from './panelStyles';
import { ContainerLink } from './ContainerLink';
import type { DGNode } from '../../types';

interface Props {
  node: DGNode;
  containers: DGNode[];
  onNavigate: (targetId: string) => void;
}

export function GhostNetworkPanel({ node, containers, onNavigate }: Props) {
  const { theme } = useTheme();
  const mono = monoStyle(theme.panelText);

  return (
    <>
      {node.driver && (
        <Section title="General">
          <Row label="Driver" value={node.driver} mono={mono} subtext={theme.nodeSubtext} />
        </Section>
      )}
      {containers.length > 0 && (
        <Section title={`Containers (${containers.length})`}>
          {containers.map((c) => (
            <ContainerLink key={c.id} name={c.name} onNavigate={onNavigate} />
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
