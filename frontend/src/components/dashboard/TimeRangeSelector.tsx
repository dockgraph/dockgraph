import { memo } from "react";
import { useTheme } from "../../theme";
import type { TimeRange } from "../../hooks/useStatsHistory";

const RANGES: TimeRange[] = ["5m", "1h", "6h", "24h"];

interface Props {
  value: TimeRange;
  onChange: (range: TimeRange) => void;
}

export const TimeRangeSelector = memo(function TimeRangeSelector({ value, onChange }: Props) {
  const { theme } = useTheme();

  return (
    <div style={{
      display: "flex",
      background: theme.panelBg,
      border: `1px solid ${theme.panelBorder}`,
      borderRadius: 6,
      padding: 2,
      gap: 2,
    }}>
      {RANGES.map(r => {
        const active = r === value;
        return (
          <button
            key={r}
            onClick={active ? undefined : () => onChange(r)}
            style={{
              padding: "4px 12px",
              borderRadius: 4,
              border: "none",
              background: active ? theme.panelBorder : "transparent",
              color: active ? theme.nodeText : theme.nodeSubtext,
              fontSize: 11,
              fontWeight: active ? 600 : 400,
              cursor: active ? "default" : "pointer",
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
