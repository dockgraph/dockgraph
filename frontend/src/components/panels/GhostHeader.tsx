import type { Theme } from '../../theme';
import type { DGNode } from '../../types';

interface Props {
  node: DGNode;
  theme: Theme;
}

/** Header for ghost (not-running) resource detail panels. */
export function GhostHeader({ node, theme }: Props) {
  return (
    <>
      <div style={{ fontSize: 16, fontWeight: 600, color: theme.nodeText, marginBottom: 2, wordBreak: 'break-all' as const }}>
        {node.name}
      </div>
      {node.image && (
        <div style={{ fontSize: 11, color: theme.nodeSubtext, marginBottom: 6, wordBreak: 'break-all' as const }}>
          {node.image}
        </div>
      )}
      <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
        <span style={{ width: 8, height: 8, borderRadius: '50%', background: theme.nodeSubtext }} />
        <span style={{ fontSize: 12, color: theme.panelText }}>Not Running</span>
        {node.source && (
          <span style={{ fontSize: 10, color: theme.nodeSubtext }}>from {node.source}</span>
        )}
      </div>
    </>
  );
}
