import type { Theme } from '../../theme';
import type { DGNode } from '../../types';
import { ContainerLink } from './ContainerLink';

interface Props {
  containers: DGNode[];
  onNavigate: (id: string) => void;
  theme: Theme;
}

/** Lists containers as clickable links, used in network/group detail panels. */
export function ContainerList({ containers, onNavigate, theme }: Props) {
  if (containers.length === 0) return null;
  return (
    <div>
      <div style={{ fontSize: 10, fontWeight: 600, color: theme.nodeSubtext, textTransform: 'uppercase', letterSpacing: '0.5px', marginBottom: 6 }}>
        Containers ({containers.length})
      </div>
      {containers.map((c) => (
        <div key={c.id} style={{ padding: '4px 0' }}>
          <ContainerLink name={c.name} onNavigate={onNavigate} />
        </div>
      ))}
    </div>
  );
}
