/** Style for clickable cross-reference links in detail panels. */
export function navLinkStyle(borderColor: string): React.CSSProperties {
  return { cursor: 'pointer', textDecoration: 'underline', textDecorationColor: borderColor, textUnderlineOffset: 2 };
}

/** Monospace text style used across detail panel values. */
export function monoStyle(panelText: string): React.CSSProperties {
  return { fontFamily: 'monospace', fontSize: 11, color: panelText, wordBreak: 'break-all' };
}
