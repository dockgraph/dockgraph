import { useEffect, useState } from 'react';
import { useTheme } from '../theme';

export function LogoutButton() {
  const { theme } = useTheme();
  const [visible, setVisible] = useState(false);

  // When auth is disabled, /api/auth/check falls through to the SPA handler
  // which returns text/html. Check Content-Type to distinguish from the actual
  // JSON auth response.
  useEffect(() => {
    fetch('/api/auth/check')
      .then((res) => {
        if (!res.ok) return false;
        const ct = res.headers.get('content-type') || '';
        return ct.includes('application/json');
      })
      .then((enabled) => setVisible(enabled))
      .catch(() => setVisible(false));
  }, []);

  if (!visible) return null;

  const handleLogout = async () => {
    try {
      await fetch('/api/logout', { method: 'POST' });
    } finally {
      window.location.reload();
    }
  };

  return (
    <button
      onClick={handleLogout}
      title="Sign out"
      aria-label="Sign out"
      style={{
        background: theme.panelBg,
        border: `1px solid ${theme.panelBorder}`,
        borderRadius: 8,
        padding: '8px 8px',
        cursor: 'pointer',
        display: 'flex',
        alignItems: 'center',
        gap: 4,
        fontSize: 12,
        lineHeight: 1,
        color: theme.panelText,
      }}
    >
      <span>Sign out</span>
    </button>
  );
}
