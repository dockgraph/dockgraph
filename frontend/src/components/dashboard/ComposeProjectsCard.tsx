import { useMemo, useState, memo } from "react";
import { useTheme } from "../../theme";
import { DashboardCard } from "./DashboardCard";
import { ProgressBar } from "./ProgressBar";
import { STATUS_COLORS } from "./palette";
import { listStacks, STANDALONE_STACK } from "../../utils/stack";
import type { DGNode } from "../../types";

interface Props {
  nodes: DGNode[];
  /** Swarm mode: the card lists stacks as well as compose projects. */
  swarm?: boolean;
  /** Scope the whole UI to the clicked stack/project (STANDALONE_STACK for none). */
  onSelectStack?: (stack: string) => void;
}

/** Shared reset so the clickable rows read as plain content, not buttons. */
const drillButton: React.CSSProperties = {
  appearance: "none",
  border: "none",
  background: "transparent",
  font: "inherit",
  color: "inherit",
  textAlign: "left",
  cursor: "pointer",
};

export const ComposeProjectsCard = memo(function ComposeProjectsCard({ nodes, swarm = false, onSelectStack }: Props) {
  const { theme } = useTheme();
  const [hovered, setHovered] = useState<string | null>(null);

  // Largest first; ties keep listStacks' name order.
  const projects = useMemo(
    () => listStacks(nodes).sort((a, b) => b.total - a.total),
    [nodes],
  );

  return (
    <DashboardCard
      title={swarm ? "Stacks / Projects" : "Compose Projects"}
      emptyMessage={projects.length === 0 ? "No containers" : undefined}
    >
      <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
        {projects.map(p => {
          const allRunning = p.running === p.total;
          const allStopped = p.running === 0;
          const statusColor = allRunning ? STATUS_COLORS.green : allStopped ? STATUS_COLORS.red : STATUS_COLORS.amber;
          const pct = p.total > 0 ? (p.running / p.total) * 100 : 0;
          const label = p.name === STANDALONE_STACK ? "standalone" : p.name;

          const content = (
            <>
              <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 4 }}>
                <span style={{
                  width: 7,
                  height: 7,
                  borderRadius: "50%",
                  background: statusColor,
                  flexShrink: 0,
                }} />
                <span style={{
                  fontSize: 12,
                  color: theme.nodeText,
                  fontWeight: 500,
                  flex: 1,
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                  whiteSpace: "nowrap",
                }}>
                  {label}
                </span>
                <span style={{ fontSize: 11, color: theme.nodeSubtext, fontFamily: "var(--dg-font-mono)", flexShrink: 0 }}>
                  {p.running}/{p.total}
                </span>
              </div>
              <ProgressBar percent={pct} color={statusColor} />
            </>
          );

          if (!onSelectStack) return <div key={p.name}>{content}</div>;

          return (
            <button
              key={p.name}
              type="button"
              onClick={() => onSelectStack(p.name)}
              onMouseEnter={() => setHovered(p.name)}
              onMouseLeave={() => setHovered(null)}
              title={`Show only ${label}`}
              style={{
                ...drillButton,
                display: "block",
                width: "calc(100% + 12px)",
                margin: "0 -6px",
                padding: "2px 6px",
                borderRadius: 5,
                background: hovered === p.name ? theme.rowHover : "transparent",
                transition: "background 0.12s",
              }}
            >
              {content}
            </button>
          );
        })}
      </div>
    </DashboardCard>
  );
});
