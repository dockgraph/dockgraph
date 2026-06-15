import type { Theme } from '../../theme';
import { Copyable } from './Copyable';

interface Props {
  name: string;
  subtitle?: string;
  theme: Theme;
}

/** Shared header for network, volume, and group detail panels. */
export function ResourceHeader({ name, subtitle, theme }: Props) {
  return (
    <>
      <div style={{ fontSize: 16, fontWeight: 600, color: theme.nodeText, marginBottom: 2, wordBreak: 'break-all' as const }}>
        <Copyable value={name}>{name}</Copyable>
      </div>
      {subtitle && (
        <div style={{ fontSize: 11, color: theme.nodeSubtext, marginBottom: 6 }}>
          {subtitle}
        </div>
      )}
    </>
  );
}
