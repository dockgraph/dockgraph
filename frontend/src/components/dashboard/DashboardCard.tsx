import { type ReactNode, memo } from "react";
import { useTheme, type Theme } from "../../theme";

interface Props {
  title: string;
  children?: ReactNode;
  /** Render a small badge/count next to the title. */
  badge?: ReactNode;
  /** Show a centered loading indicator instead of children. */
  loading?: boolean;
  /** Show a centered empty-state message instead of children. */
  emptyMessage?: string;
}

const styles = (theme: Theme) => ({
  card: {
    background: theme.panelBg,
    border: `1px solid ${theme.panelBorder}`,
    borderRadius: 8,
    display: "flex",
    flexDirection: "column" as const,
    minWidth: 0,
    overflow: "hidden" as const,
  },
  header: {
    display: "flex",
    alignItems: "center" as const,
    justifyContent: "space-between" as const,
    padding: "10px 14px",
    borderBottom: `1px solid ${theme.panelBorder}`,
    flexShrink: 0,
  },
  title: {
    fontSize: 11,
    fontWeight: 600,
    color: theme.nodeSubtext,
    textTransform: "uppercase" as const,
    letterSpacing: "0.05em",
    margin: 0,
    lineHeight: 1,
  },
  body: {
    padding: 14,
    flex: 1,
    minHeight: 0,
  },
  placeholder: {
    display: "flex",
    alignItems: "center" as const,
    justifyContent: "center" as const,
    height: "100%",
    fontSize: 11,
    color: theme.nodeSubtext,
    opacity: 0.6,
  },
});

export const DashboardCard = memo(function DashboardCard({ title, badge, children, loading, emptyMessage }: Props) {
  const { theme } = useTheme();
  const s = styles(theme);

  const showPlaceholder = loading || emptyMessage;

  return (
    <div style={s.card}>
      <div style={s.header}>
        <p style={s.title}>{title}</p>
        {badge}
      </div>
      <div style={s.body}>
        {showPlaceholder
          ? <div style={s.placeholder}>{loading ? "Loading\u2026" : emptyMessage}</div>
          : children}
      </div>
    </div>
  );
});
