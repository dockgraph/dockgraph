import type { ReactNode } from 'react';
import { useTheme } from '../theme';

/** Accent spinner used for loading states. */
export function Spinner({ size = 18 }: { size?: number }) {
  return <div className="dg-spinner" style={{ width: size, height: size }} role="status" aria-label="Loading" />;
}

interface StateProps {
  /** Optional leading glyph (small SVG/icon). */
  icon?: ReactNode;
  message: string;
  /** Show the spinner instead of an icon. */
  loading?: boolean;
}

/**
 * Centered loading / empty state for cards, panels, and charts — a spinner or
 * a muted glyph above a short message, replacing bare text placeholders.
 */
export function StateDisplay({ icon, message, loading }: StateProps) {
  const { theme } = useTheme();
  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        gap: 9,
        height: '100%',
        minHeight: 64,
        padding: 12,
        textAlign: 'center',
      }}
    >
      {loading ? <Spinner /> : icon && <span style={{ color: theme.nodeSubtext, opacity: 0.7, lineHeight: 0 }}>{icon}</span>}
      <span style={{ fontSize: 11, color: theme.nodeSubtext }}>{message}</span>
    </div>
  );
}
