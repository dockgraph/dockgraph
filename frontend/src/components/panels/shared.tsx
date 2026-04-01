import type { ReactNode } from 'react';
import { useTheme } from '../../theme';

// Re-export navLinkStyle so existing imports from './shared' keep working.
// eslint-disable-next-line react-refresh/only-export-components
export { navLinkStyle } from './panelStyles';

/** Collapsible section with uppercase title. */
export function Section({ title, children }: { title: string; children: ReactNode }) {
  const { theme } = useTheme();
  return (
    <div style={{ marginBottom: 14 }}>
      <div style={{ fontSize: 11, fontWeight: 600, color: theme.nodeSubtext, textTransform: 'uppercase', letterSpacing: 0.5, marginBottom: 6 }}>{title}</div>
      {children}
    </div>
  );
}

/** Single label: value row. */
export function Row({ label, value, mono, subtext }: { label: string; value: string; mono: React.CSSProperties; subtext: string }) {
  return (
    <div style={{ fontSize: 11, marginBottom: 3 }}>
      <span style={{ color: subtext }}>{label}: </span>
      <span style={mono}>{value}</span>
    </div>
  );
}
