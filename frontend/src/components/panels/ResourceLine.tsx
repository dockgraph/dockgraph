import { useTheme } from '../../theme';

interface Props {
  label: string;
  /** Total capacity, e.g. "4 cores" or "8 GB". */
  capacity: string;
  /** Amount in use, shown before the capacity when known. */
  used?: string;
  percent?: number;
  color: string;
}

/** Capacity line of a detail panel: label, usage figures and a usage bar. */
export function ResourceLine({ label, capacity, used, percent, color }: Props) {
  const { theme } = useTheme();
  return (
    <div style={{ marginBottom: 7 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 11, marginBottom: 3 }}>
        <span style={{ color: theme.nodeSubtext }}>{label}</span>
        <span style={{ fontFamily: 'var(--dg-font-mono)', color: theme.panelText }}>
          {percent !== undefined ? `${percent.toFixed(0)}% · ` : ''}{used ? `${used} / ` : ''}{capacity}
        </span>
      </div>
      <div style={{ height: 4, background: theme.portBg, borderRadius: 2, overflow: 'hidden' }}>
        <div style={{ width: `${percent ?? 0}%`, height: '100%', background: color, borderRadius: 2 }} />
      </div>
    </div>
  );
}
