/** Style for clickable cross-reference links in detail panels. */
export function navLinkStyle(borderColor: string): React.CSSProperties {
  return { cursor: 'pointer', textDecoration: 'underline', textDecorationColor: borderColor, textUnderlineOffset: 2 };
}

/**
 * Resets a <button>'s native look so a cross-reference link can be a real
 * button (focusable, activated by Enter/Space) while reading as text.
 */
export const navButtonReset: React.CSSProperties = {
  background: 'none',
  border: 'none',
  padding: 0,
  font: 'inherit',
  color: 'inherit',
};

/** Monospace text style used across detail panel values. */
export function monoStyle(panelText: string): React.CSSProperties {
  return { fontFamily: 'var(--dg-font-mono)', fontSize: 11, color: panelText, wordBreak: 'break-all' };
}
