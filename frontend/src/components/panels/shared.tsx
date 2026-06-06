import { useState, type ReactNode } from 'react';
import { useTheme } from '../../theme';

// Re-export panel styles so existing imports from './shared' keep working.
// eslint-disable-next-line react-refresh/only-export-components
export { navLinkStyle, monoStyle } from './panelStyles';

/** Collapsible section with a mono uppercase title and a fold toggle. */
export function Section({ title, children, defaultOpen = true }: { title: string; children: ReactNode; defaultOpen?: boolean }) {
  const { theme } = useTheme();
  const [open, setOpen] = useState(defaultOpen);

  return (
    <div style={{ marginBottom: 14 }}>
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        aria-expanded={open}
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 6,
          width: '100%',
          background: 'none',
          border: 'none',
          padding: 0,
          marginBottom: 7,
          cursor: 'pointer',
          fontFamily: 'var(--dg-font-mono)',
          fontSize: 10,
          fontWeight: 600,
          color: theme.nodeSubtext,
          textTransform: 'uppercase',
          letterSpacing: '0.08em',
        }}
      >
        <span
          aria-hidden="true"
          style={{ display: 'inline-block', fontSize: 8, transition: 'transform 0.15s', transform: open ? 'rotate(90deg)' : 'rotate(0deg)' }}
        >
          ▶
        </span>
        {title}
      </button>
      {open && children}
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
