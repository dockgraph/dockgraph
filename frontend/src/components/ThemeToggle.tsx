import { useTheme } from '../theme';

export function ThemeToggle() {
  const { theme, toggle } = useTheme();
  const isDark = theme.mode === 'dark';

  return (
    <button
      onClick={toggle}
      title={isDark ? 'Switch to light theme' : 'Switch to dark theme'}
      aria-label={isDark ? 'Switch to light theme' : 'Switch to dark theme'}
      style={{
        position: 'absolute',
        top: 10,
        right: 10,
        zIndex: 10,
        background: theme.panelBg,
        border: `1px solid ${theme.panelBorder}`,
        borderRadius: 6,
        padding: '4px 8px',
        cursor: 'pointer',
        fontSize: 14,
        lineHeight: 1,
        color: theme.panelText,
        display: 'flex',
        alignItems: 'center',
        gap: 4,
        transition: 'background 0.2s, border-color 0.2s',
      }}
    >
      <span style={{ fontSize: 12 }}>{isDark ? '\u2600' : '\u263E'}</span>
      <span style={{ fontSize: 10 }}>{isDark ? 'Light' : 'Dark'}</span>
    </button>
  );
}
