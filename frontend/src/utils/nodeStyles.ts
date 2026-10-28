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

/** Faint lift shared by graph cards (containers, per-node service cards). */
export const CARD_SHADOW = '0 2px 6px -3px rgba(0, 0, 0, 0.45)';

/** Corner radius shared by graph cards. */
export const CARD_RADIUS = 6;

/** CSS custom property holding the monospace font stack (see fonts.css). */
export const MONO_FONT_VAR = '--dg-font-mono';

const monoLabel = (fontSize: number) => ({ fontFamily: `var(${MONO_FONT_VAR})`, fontSize, fontWeight: 600 }) as const;

/**
 * Fonts of the graph node names and the service replica badge. The network
 * layout measures names in these fonts to size the nodes (layout/elk.ts),
 * so changing one here resizes the nodes to match.
 */
export const NODE_NAME_FONT = monoLabel(12.5);
export const VOLUME_NAME_FONT = monoLabel(11.5);
export const REPLICA_BADGE_FONT = monoLabel(10);
