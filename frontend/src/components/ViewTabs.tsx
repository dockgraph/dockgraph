import { useTheme } from "../theme";

const VIEWS = [
  { key: "graph", label: "Graph" },
  { key: "table", label: "Table" },
  { key: "dashboard", label: "Dashboard" },
] as const;

export type ViewKey = (typeof VIEWS)[number]["key"];

interface Props {
  activeView: ViewKey;
  onViewChange: (view: ViewKey) => void;
}

export function ViewTabs({ activeView, onViewChange }: Props) {
  const { theme } = useTheme();

  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        background: theme.panelBg,
        border: `1px solid ${theme.panelBorder}`,
        borderRadius: 8,
        padding: 3,
        gap: 2,
      }}
    >
      {VIEWS.map(({ key, label }) => {
        const active = key === activeView;
        return (
          <button
            key={key}
            onClick={active ? undefined : () => onViewChange(key)}
            aria-current={active ? "page" : undefined}
            style={{
              padding: "5px 14px",
              borderRadius: 6,
              border: "none",
              background: active ? theme.panelBorder : "transparent",
              color: active ? theme.nodeText : theme.nodeSubtext,
              fontSize: 11,
              fontWeight: active ? 500 : 400,
              cursor: active ? "default" : "pointer",
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
