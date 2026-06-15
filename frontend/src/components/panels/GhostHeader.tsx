import type { Theme } from '../../theme';
import type { DGNode } from '../../types';
import { Copyable } from './Copyable';
import { StatusBadge } from './StatusBadge';

interface Props {
  node: DGNode;
  theme: Theme;
}

/** Header for ghost (not-running) resource detail panels. */
export function GhostHeader({ node, theme }: Props) {
  return (
    <>
      <div style={{ fontSize: 16, fontWeight: 600, color: theme.nodeText, marginBottom: 2, wordBreak: 'break-all' as const }}>
        <Copyable value={node.name}>{node.name}</Copyable>
      </div>
      {node.image && (
        <div style={{ fontSize: 11, color: theme.nodeSubtext, marginBottom: 6, wordBreak: 'break-all' as const }}>
          <Copyable value={node.image}>{node.image}</Copyable>
        </div>
      )}
      <div style={{ display: 'flex', alignItems: 'center', gap: 7, flexWrap: 'wrap' }}>
        <StatusBadge status={node.status ?? 'not_running'} />
        {node.source && (
          <span style={{ fontFamily: 'var(--dg-font-mono)', fontSize: 11, color: theme.nodeSubtext }}>from {node.source}</span>
        )}
      </div>
    </>
  );
}
