import type { CSSProperties } from 'react';

/**
 * Shared small rounded-rect badge look: SwarmNodeBadges' role/availability/
 * state chips and ControlSummary's protocol badge all use this. Kept in its
 * own module (rather than exported from a component file) so React Fast
 * Refresh stays happy — a component file may only export components.
 */
export const chip: CSSProperties = {
  flexShrink: 0,
  fontFamily: 'var(--dg-font-mono)',
  fontSize: 9,
  fontWeight: 600,
  textTransform: 'uppercase',
  letterSpacing: '0.05em',
  borderRadius: 3,
  padding: '0 4px',
  lineHeight: '13px',
  whiteSpace: 'nowrap',
};
