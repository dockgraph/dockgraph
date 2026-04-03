import type { Theme } from "../../theme";

export const tableLayout = (theme: Theme) => ({
  container: {
    display: "flex",
    flexDirection: "column" as const,
    height: "100%",
    background: theme.canvasBg,
    color: theme.panelText,
    fontFamily: "system-ui, -apple-system, sans-serif",
  },
  scrollBody: {
    flex: 1,
    overflowY: "auto" as const,
    padding: "0 16px 16px",
  },
  toolbar: {
    display: "flex",
    alignItems: "center",
    gap: 12,
    padding: "12px 16px",
    fontSize: 12,
  },
  card: {
    borderRadius: 8,
    overflow: "hidden" as const,
    border: `1px solid ${theme.panelBorder}`,
    background: theme.cardBg,
    flexShrink: 0,
  },
});

export const tableRow = (theme: Theme) => ({
    header: {
      display: "grid",
      padding: "10px 16px",
      fontSize: 11,
      fontWeight: 600 as const,
      color: theme.nodeSubtext,
      textTransform: "uppercase" as const,
      letterSpacing: 0.5,
      borderBottom: `1px solid ${theme.panelBorder}`,
      position: "sticky" as const,
      top: 0,
      background: theme.panelBg,
      zIndex: 2,
    },
    row: {
      display: "grid",
      padding: "10px 16px",
      fontSize: 13,
      color: theme.nodeText,
      borderBottom: `1px solid ${theme.panelBorder}`,
      cursor: "pointer",
      alignItems: "center",
      outline: "none",
    },
    statsRow: {
      display: "flex",
      gap: 24,
      padding: "4px 16px 8px",
      fontSize: 11,
      color: theme.nodeSubtext,
      borderBottom: `1px solid ${theme.panelBorder}`,
      background: theme.statsRowBg,
    },
    statsValue: (color: string) => ({
      color,
      fontFamily: "monospace",
    }),
});

export const groupHeader = (theme: Theme) => ({
  container: {
    display: "flex",
    alignItems: "center",
    gap: 8,
    padding: "10px 16px",
    fontSize: 12,
    fontWeight: 600 as const,
    color: theme.nodeText,
    background: theme.panelBg,
    borderBottom: `1px solid ${theme.panelBorder}`,
    cursor: "pointer",
    userSelect: "none" as const,
  },
  chevron: (collapsed: boolean) => ({
    display: "inline-block",
    transition: "transform 0.15s",
    transform: collapsed ? "rotate(0deg)" : "rotate(90deg)",
    fontSize: 10,
  }),
  count: {
    fontSize: 11,
    color: theme.nodeSubtext,
    fontWeight: 400 as const,
  },
});

export const resourceTabs = (theme: Theme) => ({
  container: {
    display: "flex",
    gap: 0,
    borderBottom: `1px solid ${theme.panelBorder}`,
    padding: "0 16px",
  },
  tab: (active: boolean) => ({
    padding: "10px 16px",
    fontSize: 13,
    fontWeight: active ? 600 : 400,
    color: active ? theme.nodeText : theme.nodeSubtext,
    background: "transparent",
    border: "none",
    borderBottom: active ? `2px solid ${theme.nodeText}` : "2px solid transparent",
    cursor: active ? "default" : "pointer",
    transition: "color 0.15s",
  }),
});

export const CONTAINER_GRID = "2fr 2fr 100px 140px 140px";
export const NETWORK_GRID = "2fr 1fr 1fr 1fr 80px";
export const VOLUME_GRID = "2fr 1fr 2fr 80px";
