import { memo, useState, useRef, useEffect } from "react";
import { useTheme } from "../../theme";
import { tableLayout } from "./tableStyles";
import type { GroupByKey } from "../../hooks/useTableGrouping";

interface Props {
  groupBy: GroupByKey;
  onGroupByChange: (key: GroupByKey) => void;
}

const GROUP_OPTIONS: { key: GroupByKey; label: string }[] = [
  { key: "compose", label: "Compose Project" },
  { key: "network", label: "Network" },
  { key: "status", label: "Status" },
  { key: "none", label: "None" },
];

export const TableToolbar = memo(function TableToolbar({
  groupBy,
  onGroupByChange,
}: Props) {
  const { theme } = useTheme();
  const styles = tableLayout(theme);
  const [open, setOpen] = useState(false);
  const [hoveredKey, setHoveredKey] = useState<GroupByKey | null>(null);
  const ref = useRef<HTMLDivElement>(null);

  const activeLabel = GROUP_OPTIONS.find((o) => o.key === groupBy)?.label ?? groupBy;

  useEffect(() => {
    if (!open) return;
    const handleClick = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, [open]);

  return (
    <div style={styles.toolbar}>
      <label style={{ color: theme.nodeSubtext, fontSize: 12 }}>
        Group by:
      </label>
      <div ref={ref} style={{ position: "relative" }}>
        <button
          aria-haspopup="listbox"
          aria-expanded={open}
          aria-label="Group by"
          onClick={() => setOpen((v) => !v)}
          style={{
            background: theme.panelBg,
            color: theme.panelText,
            border: `1px solid ${theme.panelBorder}`,
            borderRadius: 6,
            padding: "5px 28px 5px 10px",
            fontSize: 12,
            cursor: "pointer",
            outline: "none",
            position: "relative",
            textAlign: "left",
          }}
        >
          {activeLabel}
          <span
            style={{
              position: "absolute",
              right: 9,
              top: "50%",
              marginTop: -5,
              pointerEvents: "none",
              color: theme.nodeSubtext,
              fontSize: 10,
            }}
          >
            {open ? "▲" : "▼"}
          </span>
        </button>
        {open && (
          <div
            role="listbox"
            aria-label="Group by options"
            style={{
              position: "absolute",
              top: "calc(100% + 4px)",
              left: 0,
              minWidth: "100%",
              background: theme.panelBg,
              border: `1px solid ${theme.panelBorder}`,
              borderRadius: 6,
              overflow: "hidden",
              zIndex: 20,
              boxShadow: theme.mode === "dark"
                ? "0 4px 12px rgba(0,0,0,0.4)"
                : "0 4px 12px rgba(0,0,0,0.1)",
            }}
          >
            {GROUP_OPTIONS.map((opt) => {
              const isActive = opt.key === groupBy;
              const isHovered = opt.key === hoveredKey;
              let bg = "transparent";
              if (isActive) bg = theme.panelBorder;
              else if (isHovered) bg = theme.rowHover;

              return (
                <div
                  key={opt.key}
                  role="option"
                  aria-selected={isActive}
                  onClick={() => { onGroupByChange(opt.key); setOpen(false); }}
                  onMouseEnter={() => setHoveredKey(opt.key)}
                  onMouseLeave={() => setHoveredKey(null)}
                  style={{
                    padding: "6px 12px",
                    fontSize: 12,
                    cursor: "pointer",
                    color: isActive ? theme.nodeText : theme.panelText,
                    background: bg,
                    whiteSpace: "nowrap",
                  }}
                >
                  {opt.label}
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
});
