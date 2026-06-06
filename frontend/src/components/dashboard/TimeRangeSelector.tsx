import { memo, useState } from "react";
import { useTheme } from "../../theme";
import type { TimeRange } from "../../hooks/useStatsHistory";

const RANGES: TimeRange[] = ["5m", "1h", "6h", "24h"];

interface Props {
  value: TimeRange;
  onChange: (range: TimeRange) => void;
}

export const TimeRangeSelector = memo(function TimeRangeSelector({ value, onChange }: Props) {
  const { theme } = useTheme();
  const [hovered, setHovered] = useState<TimeRange | null>(null);

  return (
    <div style={{
      display: "flex",
      // Recessed track so the active chip reads as raised (matches view tabs).
      background: theme.canvasBg,
      border: `1px solid ${theme.panelBorder}`,
      borderRadius: 8,
      padding: 3,
      gap: 2,
    }}>
      {RANGES.map(r => {
        const active = r === value;
        const hot = !active && hovered === r;
        return (
          <button
            key={r}
            onClick={active ? undefined : () => onChange(r)}
            onMouseEnter={() => setHovered(r)}
            onMouseLeave={() => setHovered((h) => (h === r ? null : h))}
            style={{
              padding: "5px 12px",
              borderRadius: 6,
              border: "none",
              fontFamily: "var(--dg-font-mono)",
              fontSize: 11,
              fontWeight: active ? 600 : 500,
              cursor: active ? "default" : "pointer",
              background: active ? theme.nodeBg : hot ? theme.rowHover : "transparent",
              color: active ? theme.accent : hot ? theme.nodeText : theme.nodeSubtext,
              boxShadow: active ? `inset 0 0 0 1px ${theme.accentSoft}, 0 1px 2px rgba(0, 0, 0, 0.25)` : "none",
              transition: "background 0.15s, color 0.15s",
              lineHeight: 1,
            }}
          >
            {r}
          </button>
        );
      })}
    </div>
  );
});
