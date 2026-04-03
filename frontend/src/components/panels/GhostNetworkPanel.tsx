import { useTheme } from '../../theme';
import { Section, Row } from './shared';
import { monoStyle, navLinkStyle } from './panelStyles';
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
            <div
              key={c.id}
              style={{
                fontSize: 11,
                fontWeight: 600,
                color: theme.panelText,
                marginBottom: 6,
                ...navLinkStyle(theme.panelBorder),
              }}
              onClick={() => onNavigate(`container:${c.name}`)}
              title={`Inspect ${c.name}`}
            >
              {c.name}
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
