import { useTheme } from '../../theme';
import { Section } from './shared';
import { Copyable } from './Copyable';
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
      {node.compose && <DetailPanelCompose compose={node.compose} onNavigate={onNavigate} />}
      {node.ports && node.ports.length > 0 && (
        <Section title="Ports">
          {node.ports.map((p, i) => {
            const proto = p.protocol ? `/${p.protocol}` : '';
            return (
              <Copyable
                key={i}
                value={`${p.host}:${p.container}${proto}`}
                style={{ display: 'block', fontSize: 11, color: theme.panelText, marginBottom: 2 }}
              >
                <span style={{ fontFamily: 'var(--dg-font-mono)' }}>{p.host}</span>
                <span style={{ color: theme.nodeSubtext }}> → </span>
                <span style={{ fontFamily: 'var(--dg-font-mono)' }}>{p.container}{proto}</span>
              </Copyable>
            );
          })}
        </Section>
      )}
    </>
  );
}
