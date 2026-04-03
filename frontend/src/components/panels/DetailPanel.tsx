import { useEffect, type ReactNode } from 'react';
import { useTheme } from '../../theme';
import { DETAIL_PANEL_WIDTH } from '../../utils/constants';

interface DetailPanelProps {
  open: boolean;
  onClose: () => void;
  header?: ReactNode;
  loading?: boolean;
  error?: string | null;
  children: ReactNode;
}

export function DetailPanel({ open, onClose, header, loading, error, children }: DetailPanelProps) {
  const { theme } = useTheme();

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && open) onClose();
    };
    document.addEventListener('keydown', handler);
    return () => document.removeEventListener('keydown', handler);
  }, [open, onClose]);

  return (
    <div
      style={{
        position: 'absolute',
        top: 0,
        right: 0,
        width: DETAIL_PANEL_WIDTH,
        height: '100%',
        background: theme.panelBg,
        borderLeft: `1px solid ${theme.panelBorder}`,
        zIndex: 20,
        transform: open ? 'translateX(0)' : `translateX(${DETAIL_PANEL_WIDTH}px)`,
        transition: 'transform 0.2s ease-out',
        display: 'flex',
        flexDirection: 'column',
        overflow: 'hidden',
      }}
    >
      {/* Fixed header: close button + container identity */}
      <div style={{ padding: '12px 16px 8px', borderBottom: `1px solid ${theme.panelBorder}`, flexShrink: 0 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
          <div style={{ flex: 1, minWidth: 0 }}>{header}</div>
          <button
            onClick={onClose}
            aria-label="Close panel"
            style={{
              background: 'none',
              border: 'none',
              color: theme.panelText,
              fontSize: 18,
              cursor: 'pointer',
              lineHeight: 1,
              padding: 4,
              flexShrink: 0,
              marginLeft: 8,
            }}
          >
            ✕
          </button>
        </div>
      </div>

      {/* Scrollable body */}
      {loading && (
        <div style={{ padding: 20, color: theme.nodeSubtext, textAlign: 'center' }}>Loading…</div>
      )}
      {error && (
        <div style={{ padding: 20, textAlign: 'center' }}>
          <div style={{ color: theme.danger, marginBottom: 8 }}>{error}</div>
        </div>
      )}
      {!loading && !error && (
        <div style={{ flex: 1, overflowY: 'auto', padding: '12px 16px' }}>
          {children}
        </div>
      )}
    </div>
  );
}
