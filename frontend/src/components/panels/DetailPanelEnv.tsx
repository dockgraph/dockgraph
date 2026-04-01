import { useState } from 'react';
import { useTheme } from '../../theme';
import { Section } from './DetailPanelStats';
import type { ContainerDetail } from '../../types/stats';

interface Props {
  env: ContainerDetail['env'];
}

export function DetailPanelEnv({ env }: Props) {
  const { theme } = useTheme();
  const [revealed, setRevealed] = useState<Set<number>>(new Set());
  if (!env?.length) return null;

  const toggle = (i: number) => setRevealed((prev) => {
    const next = new Set(prev);
    next.has(i) ? next.delete(i) : next.add(i);
    return next;
  });

  return (
    <Section title="Environment">
      {env.map((e, i) => {
        const masked = e.value === '********';
        return (
          <div key={i} style={{ fontSize: 11, marginBottom: 3, display: 'flex', gap: 4, alignItems: 'baseline' }}>
            <span style={{ color: theme.nodeSubtext, flexShrink: 0 }}>{e.key}=</span>
            <span style={{ fontFamily: 'monospace', color: theme.panelText, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', minWidth: 0 }}>
              {masked && !revealed.has(i) ? '••••••••' : e.value}
            </span>
            {masked && (
              <button onClick={() => toggle(i)} style={{ background: 'none', border: 'none', color: theme.nodeSubtext, fontSize: 10, cursor: 'pointer', padding: 0, flexShrink: 0 }} aria-label={revealed.has(i) ? 'Hide value' : 'Show value'}>
                {revealed.has(i) ? '🔒' : '👁'}
              </button>
            )}
          </div>
        );
      })}
    </Section>
  );
}
