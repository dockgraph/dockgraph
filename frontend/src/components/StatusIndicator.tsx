import { useTheme } from '../theme';

interface StatusIndicatorProps {
  connected: boolean;
}

/** Displays a small badge showing the current WebSocket connection status. */
export function StatusIndicator({ connected }: StatusIndicatorProps) {
  const { theme } = useTheme();

  return (
    <div
      role="status"
      aria-live="polite"
      aria-label={connected ? 'Backend connected' : 'Backend disconnected'}
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: 4,
        background: theme.panelBg,
        border: `1px solid ${theme.panelBorder}`,
        borderRadius: 8,
        padding: '8px 8px',
        fontSize: 12,
        lineHeight: 1,
        color: theme.panelText,
      }}
    >
      <span
        style={{
          width: 7,
          height: 7,
          borderRadius: '50%',
          background: connected ? '#22c55e' : '#ef4444',
        }}
      />
      {connected ? 'Live' : 'Disconnected'}
    </div>
  );
}
