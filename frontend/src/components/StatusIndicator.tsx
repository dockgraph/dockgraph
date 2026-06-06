import type { CSSProperties } from 'react';
import { useTheme } from '../theme';

interface StatusIndicatorProps {
  connected: boolean;
}

/** Small badge reflecting the live WebSocket connection to the backend. */
export function StatusIndicator({ connected }: StatusIndicatorProps) {
  const { theme } = useTheme();
  const color = connected ? theme.success : theme.danger;

  // Custom props feed the @keyframes pulse (a constant glow + an expanding ring).
  const dotStyle: CSSProperties = {
    width: 7,
    height: 7,
    borderRadius: '50%',
    background: color,
    flex: '0 0 auto',
    ...(connected
      ? ({ '--dg-glow': `${color}88`, '--dg-pulse': `${color}55` } as CSSProperties)
      : { boxShadow: `0 0 5px ${color}66` }),
  };

  return (
    <div
      role="status"
      aria-live="polite"
      aria-label={connected ? 'Backend connected' : 'Backend disconnected'}
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: 7,
        background: theme.canvasBg,
        border: `1px solid ${theme.panelBorder}`,
        borderRadius: 8,
        padding: '6px 11px',
        fontFamily: 'var(--dg-font-mono)',
        fontSize: 11,
        fontWeight: 600,
        letterSpacing: '0.06em',
        lineHeight: 1,
        color: connected ? theme.nodeSubtext : theme.danger,
      }}
    >
      <span className={connected ? 'dg-live-dot' : undefined} style={dotStyle} />
      {connected ? 'LIVE' : 'OFFLINE'}
    </div>
  );
}
