import { useTheme } from '../../theme';
import { Section } from './shared';
import { Copyable } from './Copyable';
import { DetailPanelCompose } from './DetailPanelCompose';
import { containerPorts, hostPorts, portMapping } from '../../utils/ports';
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
                value={`${portMapping(p)}${proto}`}
                style={{ display: 'block', fontSize: 11, color: theme.panelText, marginBottom: 2 }}
              >
                <span style={{ fontFamily: 'var(--dg-font-mono)' }}>{hostPorts(p)}</span>
                <span style={{ color: theme.nodeSubtext }}> → </span>
                <span style={{ fontFamily: 'var(--dg-font-mono)' }}>{containerPorts(p)}{proto}</span>
              </Copyable>
            );
          })}
        </Section>
      )}
    </>
  );
}
