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
    <div style={{ display: "flex", gap: 4, alignItems: "center" }}>
      {RANGES.map(r => {
        const active = r === value;
        return (
          <button
            key={r}
            onClick={active ? undefined : () => onChange(r)}
            style={{
              padding: "4px 10px",
              borderRadius: 4,
              border: `1px solid ${theme.panelBorder}`,
              background: active ? theme.panelBorder : "transparent",
              color: active ? theme.nodeText : theme.nodeSubtext,
              fontSize: 11,
              fontWeight: active ? 600 : 400,
              cursor: active ? "default" : "pointer",
            }}
          >
            {r}
          </button>
        );
      })}
    </div>
  );
});
