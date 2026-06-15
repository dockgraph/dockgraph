import type { Theme } from '../theme';
import { STATUS_COLORS } from './colors';

/** Returns border styles for ghost (not-running) vs normal nodes. */
export function ghostBorder(isGhost: boolean, theme: Theme): React.CSSProperties {
  const style = isGhost ? 'dashed' : 'solid';
  const color = isGhost ? theme.nodeGhostBorder : theme.nodeBorder;
  const border = `1px ${style} ${color}`;
  return { borderTop: border, borderRight: border, borderBottom: border };
}

/**
 * Left-rail accent colour for a node. Not-running (ghost) nodes use the neutral
 * status colour so they read as inactive; otherwise the node's own accent shows.
 */
export function railColor(isGhost: boolean, activeColor: string): string {
  return isGhost ? STATUS_COLORS.not_running : activeColor;
}
