import { type ReactNode, memo } from "react";
import { useTheme, type Theme } from "../../theme";
import { StateDisplay } from "../StateDisplay";

interface Props {
  title: string;
  children?: ReactNode;
  /** Render a small badge/count next to the title. */
  badge?: ReactNode;
  /**
   * Swarm node the card's figures come from, for cards that read the
   * server's own daemon rather than the whole cluster.
   */
  node?: string;
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
    fontFamily: "var(--dg-font-mono)",
    fontSize: 10,
    fontWeight: 600,
    color: theme.nodeSubtext,
    textTransform: "uppercase" as const,
    letterSpacing: "0.07em",
    margin: 0,
    lineHeight: 1,
  },
  node: {
    display: "block",
    marginTop: 4,
    fontSize: 10,
    lineHeight: 1,
    color: theme.nodeSubtext,
  },
  body: {
    padding: 14,
    flex: 1,
    minHeight: 0,
  },
});

export const DashboardCard = memo(function DashboardCard({ title, badge, node, children, loading, emptyMessage }: Props) {
  const { theme } = useTheme();
  const s = styles(theme);

  const showPlaceholder = loading || emptyMessage;

  return (
    <div style={s.card}>
      <div style={s.header}>
        <div style={{ minWidth: 0 }}>
          <p style={s.title}>{title}</p>
          {node && (
            <span style={s.node} title="Only the node running DockGraph, not the whole swarm">
              on <span style={{ fontFamily: "var(--dg-font-mono)", color: theme.nodeText }}>{node}</span>
            </span>
          )}
        </div>
        {badge}
      </div>
      <div style={s.body}>
        {showPlaceholder
          ? <StateDisplay loading={loading} message={loading ? "Loading" : (emptyMessage ?? "")} />
          : children}
      </div>
    </div>
  );
});
