import { memo } from "react";
import { useTheme } from "../../theme";
import { groupHeader } from "./tableStyles";

interface Props {
  label: string;
  count: number;
  collapsed: boolean;
  onToggle: () => void;
  color?: string;
}

export const TableGroupHeader = memo(function TableGroupHeader({
  label,
  count,
  collapsed,
  onToggle,
  color,
}: Props) {
  const { theme } = useTheme();
  const styles = groupHeader(theme);

  return (
    <div
      style={styles.container}
      onClick={onToggle}
      role="button"
      tabIndex={0}
      onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") onToggle(); }}
    >
      <span data-testid="chevron" style={styles.chevron(collapsed)}>
        &#9654;
      </span>
      {color && (
        <span style={{ width: 8, height: 8, borderRadius: "50%", background: color, flexShrink: 0 }} />
      )}
      <span>{label}</span>
      <span style={styles.count}>({count})</span>
    </div>
  );
});
