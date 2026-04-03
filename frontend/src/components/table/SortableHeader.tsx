import { memo } from "react";
import { useTheme } from "../../theme";
import { tableRow } from "./tableStyles";

interface Column {
  key: string;
  label: string;
}

interface Props {
  columns: Column[];
  sortColumn: string;
  sortDirection: "asc" | "desc";
  onSort: (column: string) => void;
  gridTemplate: string;
}

export const SortableHeader = memo(function SortableHeader({
  columns,
  sortColumn,
  sortDirection,
  onSort,
  gridTemplate,
}: Props) {
  const { theme } = useTheme();
  const styles = tableRow(theme);

  return (
    <div style={{ ...styles.header, gridTemplateColumns: gridTemplate }}>
      {columns.map((col) => {
        const active = sortColumn === col.key;
        return (
          <button
            key={col.key}
            onClick={() => onSort(col.key)}
            style={{
              all: "unset",
              cursor: "pointer",
              display: "flex",
              alignItems: "center",
              gap: 4,
              color: active ? theme.nodeText : theme.nodeSubtext,
              fontWeight: active ? 700 : 600,
              fontSize: "inherit",
              textTransform: "inherit",
              letterSpacing: "inherit",
            }}
          >
            {col.label}
            {active && (
              <span style={{ fontSize: 8 }}>
                {sortDirection === "asc" ? "\u25B2" : "\u25BC"}
              </span>
            )}
          </button>
        );
      })}
    </div>
  );
});
