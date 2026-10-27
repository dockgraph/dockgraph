import { STATUS_COLORS, STATUS_LABELS } from '../../utils/colors';

interface PillProps {
  label: string;
  color: string;
  /** Glows the dot, for a live (running) status. */
  glow?: boolean;
}

/** Uppercase pill with a coloured dot, as detail panel headers show a status. */
export function Pill({ label, color, glow = false }: PillProps) {
  return (
    <span
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: 6,
        padding: '3px 9px',
        borderRadius: 999,
        background: `${color}22`,
        fontFamily: 'var(--dg-font-mono)',
        fontSize: 10,
        fontWeight: 600,
        letterSpacing: '0.04em',
        textTransform: 'uppercase',
        color,
      }}
    >
      <span style={{ width: 6, height: 6, borderRadius: '50%', background: color, flexShrink: 0, boxShadow: glow ? `0 0 6px ${color}` : 'none' }} />
      {label}
    </span>
  );
}

interface Props {
  status: string;
}

/** Pill badge showing a container/resource status with a coloured dot. */
export function StatusBadge({ status }: Props) {
  const color = STATUS_COLORS[status] ?? STATUS_COLORS.exited;
  return <Pill label={STATUS_LABELS[status] ?? status} color={color} glow={status === 'running'} />;
}
