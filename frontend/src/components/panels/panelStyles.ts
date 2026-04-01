/** Style for clickable cross-reference links in detail panels. */
export function navLinkStyle(borderColor: string): React.CSSProperties {
  return { cursor: 'pointer', textDecoration: 'underline', textDecorationColor: borderColor, textUnderlineOffset: 2 };
}
