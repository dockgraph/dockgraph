import { useTheme } from '../theme';

interface StatusIndicatorProps {
  connected: boolean;
}

/** Displays a small badge showing the current WebSocket connection status. */
export function StatusIndicator({ connected }: StatusIndicatorProps) {
  const { theme } = useTheme();

  return (
    <div
      style={{
        position: 'absolute',
        top: 10,
        left: 10,
        zIndex: 10,
        display: 'flex',
        alignItems: 'center',
        gap: 6,
        background: theme.panelBg,
        border: `1px solid ${theme.panelBorder}`,
        borderRadius: 6,
        padding: '4px 10px',
        fontSize: 11,
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
