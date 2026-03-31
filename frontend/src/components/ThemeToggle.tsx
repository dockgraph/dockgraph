import { useTheme } from '../theme';

export function ThemeToggle() {
  const { theme, toggle } = useTheme();
  const isDark = theme.mode === 'dark';

  return (
    <button
      className="dg-theme-toggle"
      onClick={toggle}
      title={isDark ? 'Switch to light theme' : 'Switch to dark theme'}
      aria-label={isDark ? 'Switch to light theme' : 'Switch to dark theme'}
    >
      {isDark ? '\u2600' : '\u263E'}
    </button>
  );
}
