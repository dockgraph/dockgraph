import { useEffect, useState } from 'react';

export function LogoutButton() {
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
      className="dg-iconbtn"
    >
      <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
        <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" />
        <polyline points="16 17 21 12 16 7" />
        <line x1="21" y1="12" x2="9" y2="12" />
      </svg>
    </button>
  );
}
