import { useState } from "react";
import { useTheme } from "../theme";

const VIEWS = [
  { key: "graph", label: "Graph" },
  { key: "table", label: "Table" },
  { key: "dashboard", label: "Dashboard" },
  { key: "logs", label: "Logs" },
] as const;

export type ViewKey = (typeof VIEWS)[number]["key"];

interface Props {
  activeView: ViewKey;
  onViewChange: (view: ViewKey) => void;
}

export function ViewTabs({ activeView, onViewChange }: Props) {
  const { theme } = useTheme();
  const [hovered, setHovered] = useState<ViewKey | null>(null);

  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        // Recessed track, a shade below the top bar so the active chip reads as raised.
        background: theme.canvasBg,
        border: `1px solid ${theme.panelBorder}`,
        borderRadius: 8,
        padding: 3,
        gap: 2,
      }}
    >
      {VIEWS.map(({ key, label }) => {
        const active = key === activeView;
        const hot = !active && hovered === key;
        return (
          <button
            key={key}
            onClick={active ? undefined : () => onViewChange(key)}
            onMouseEnter={() => setHovered(key)}
            onMouseLeave={() => setHovered((h) => (h === key ? null : h))}
            aria-current={active ? "page" : undefined}
            style={{
              padding: "6px 14px",
              borderRadius: 6,
              border: "none",
              fontFamily: "var(--dg-font-ui)",
              fontSize: 12,
              fontWeight: active ? 600 : 500,
              cursor: active ? "default" : "pointer",
              background: active ? theme.nodeBg : hot ? theme.rowHover : "transparent",
              color: active ? theme.accent : hot ? theme.nodeText : theme.nodeSubtext,
              boxShadow: active
                ? `inset 0 0 0 1px ${theme.accentSoft}, 0 1px 2px rgba(0, 0, 0, 0.25)`
                : "none",
              transition: "background 0.15s, color 0.15s",
            }}
          >
            {label}
          </button>
        );
      })}
    </div>
  );
}
