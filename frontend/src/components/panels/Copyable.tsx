import { useState, useCallback, type ReactNode, type CSSProperties } from 'react';
import { useTheme } from '../../theme';

interface Props {
  /** Text written to the clipboard on click. */
  value: string;
  /** Visible content; defaults to the value itself. */
  children?: ReactNode;
  style?: CSSProperties;
}

/**
 * Inline value that copies itself to the clipboard on click, flashing the
 * accent colour as confirmation. Used for IDs, env vars, ports, labels — the
 * things people open the inspector to grab.
 */
export function Copyable({ value, children, style }: Props) {
  const { theme } = useTheme();
  const [copied, setCopied] = useState(false);

  const copy = useCallback(
    (e: React.MouseEvent) => {
      e.stopPropagation();
      if (!navigator.clipboard) return;
      navigator.clipboard
        .writeText(value)
        .then(() => {
          setCopied(true);
          window.setTimeout(() => setCopied(false), 1100);
        })
        .catch(() => {});
    },
    [value],
  );

  return (
    <span
      className="dg-copy"
      onClick={copy}
      title={copied ? 'Copied' : 'Click to copy'}
      style={{ transition: 'color 0.15s', ...style, ...(copied ? { color: theme.accent } : null) }}
    >
      {children ?? value}
    </span>
  );
}
