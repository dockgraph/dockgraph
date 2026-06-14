import { useContainerLogs } from '../../hooks/useContainerLogs';
import { useTheme } from '../../theme';
import { LogStream } from '../logwindow/LogStream';

interface Props {
  containerId: string | null;
  active: boolean;
  /** Optional: open these logs in a floating window. */
  onPopOut?: () => void;
}

export function DetailPanelLogs({ containerId, active, onPopOut }: Props) {
  const logs = useContainerLogs(containerId, active);
  const { theme } = useTheme();

  return (
    <div style={{ marginTop: 14 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 6 }}>
        <span
          style={{
            fontFamily: 'var(--dg-font-mono)',
            fontSize: 10,
            fontWeight: 600,
            color: theme.nodeSubtext,
            textTransform: 'uppercase',
            letterSpacing: '0.08em',
          }}
        >
          Logs
        </span>
        <span
          style={{
            width: 6,
            height: 6,
            borderRadius: '50%',
            background: logs.connected ? theme.success : theme.danger,
            boxShadow: logs.connected ? `0 0 6px ${theme.success}` : 'none',
          }}
        />
        {onPopOut && (
          <button
            type="button"
            onClick={onPopOut}
            title="Open logs in window"
            aria-label="Open logs in window"
            style={{
              marginLeft: 'auto',
              display: 'inline-flex',
              alignItems: 'center',
              justifyContent: 'center',
              width: 20,
              height: 20,
              padding: 0,
              borderRadius: 4,
              border: `1px solid ${theme.panelBorder}`,
              background: 'transparent',
              color: theme.panelText,
              cursor: 'pointer',
            }}
          >
            <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M14 4h6v6M20 4l-8 8M10 6H6a2 2 0 0 0-2 2v10a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2v-4" />
            </svg>
          </button>
        )}
      </div>
      <div style={{ height: 250 }}>
        <LogStream {...logs} active={active} />
      </div>
    </div>
  );
}
