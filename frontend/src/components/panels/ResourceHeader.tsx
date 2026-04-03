import type { Theme } from '../../theme';

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
        {name}
      </div>
      {subtitle && (
        <div style={{ fontSize: 11, color: theme.nodeSubtext, marginBottom: 6 }}>
          {subtitle}
        </div>
      )}
    </>
  );
}
