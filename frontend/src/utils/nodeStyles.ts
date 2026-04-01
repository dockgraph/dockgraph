import type { Theme } from '../theme';

/** Returns border styles for ghost (not-running) vs normal nodes. */
export function ghostBorder(isGhost: boolean, theme: Theme): React.CSSProperties {
  const style = isGhost ? 'dashed' : 'solid';
  const color = isGhost ? theme.nodeGhostBorder : theme.nodeBorder;
  const border = `1px ${style} ${color}`;
  return { borderTop: border, borderRight: border, borderBottom: border };
}
