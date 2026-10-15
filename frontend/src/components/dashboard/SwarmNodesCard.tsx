import { useMemo, useState, memo } from "react";
import { useTheme } from "../../theme";
import { DashboardCard } from "./DashboardCard";
import { ProgressBar } from "./ProgressBar";
import { METRIC_COLORS } from "./palette";
import { SwarmRoleBadge, SwarmAvailabilityChip } from "../SwarmNodeBadges";
import { swarmNodeStateColor } from "../../utils/colors";
import { listSwarmNodes, placeTasks, nodeUsage, formatCores } from "../../utils/nodeTransform";
import { nodeStatsKey } from "../../utils/stack";
import { formatBytes } from "../../utils/format";
import type { DGNode } from "../../types";
import type { ContainerStatsData } from "../../types/stats";

interface Props {
  /** The (stack-scoped) graph: swarm nodes plus the services whose tasks are counted. */
  nodes: DGNode[];
  /** Live stats, including the per-node aggregates `node:{hostname}`. */
  statsMap: Map<string, ContainerStatsData>;
  /** Open the detail panel for the clicked swarm node. */
  onInspect?: (nodeId: string) => void;
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

/**
 * One row per swarm node: state, hostname, role (★ leader), availability,
 * task count, and CPU/memory bars against the node's capacity from its
 * agent's aggregate stats ("no agent" when none reports).
 */
export const SwarmNodesCard = memo(function SwarmNodesCard({ nodes, statsMap, onInspect }: Props) {
  const { theme } = useTheme();
  const [hovered, setHovered] = useState<string | null>(null);

  const swarmNodes = useMemo(() => listSwarmNodes(nodes), [nodes]);
  const placed = useMemo(() => placeTasks(nodes), [nodes]);

  return (
    <DashboardCard
      title="Swarm Nodes"
      emptyMessage={swarmNodes.length === 0 ? "No swarm nodes" : undefined}
    >
      <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
        {swarmNodes.map((n) => {
          const info = n.swarmNode;
          const stats = statsMap.get(nodeStatsKey(n.name));
          const usage = nodeUsage(info, stats);
          const tasks = placed.get(n.id)?.length ?? 0;
          const caption: React.CSSProperties = {
            fontFamily: "var(--dg-font-mono)",
            fontSize: 9,
            fontWeight: 600,
            color: theme.nodeSubtext,
            letterSpacing: "0.05em",
            width: 28,
            flexShrink: 0,
          };
          const value: React.CSSProperties = {
            fontFamily: "var(--dg-font-mono)",
            fontSize: 10,
            color: theme.nodeSubtext,
            width: 110,
            textAlign: "right",
            flexShrink: 0,
            whiteSpace: "nowrap",
            overflow: "hidden",
            textOverflow: "ellipsis",
          };

          const content = (
            <>
              <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 5 }}>
                <span
                  title={info?.state ?? n.status}
                  style={{
                    width: 7,
                    height: 7,
                    borderRadius: "50%",
                    background: swarmNodeStateColor(info?.state ?? n.status),
                    flexShrink: 0,
                  }}
                />
                <span style={{
                  fontSize: 12,
                  color: theme.nodeText,
                  fontWeight: 500,
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                  whiteSpace: "nowrap",
                  minWidth: 0,
                }}>
                  {n.name}
                </span>
                <SwarmRoleBadge info={info} />
                <SwarmAvailabilityChip info={info} />
                <span style={{ flex: 1 }} />
                <span style={{ fontSize: 11, color: theme.nodeSubtext, fontFamily: "var(--dg-font-mono)", flexShrink: 0 }}>
                  {tasks} {tasks === 1 ? "task" : "tasks"}
                </span>
              </div>
              {stats ? (
                <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
                  <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                    <span style={caption}>CPU</span>
                    <div style={{ flex: 1 }}>
                      <ProgressBar percent={usage.cpuPercent ?? 0} color={METRIC_COLORS.cpu} />
                    </div>
                    <span style={value}>
                      {stats.cpuPercent.toFixed(1)}%{usage.cores ? ` / ${formatCores(usage.cores)}` : ""}
                    </span>
                  </div>
                  <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                    <span style={caption}>MEM</span>
                    <div style={{ flex: 1 }}>
                      <ProgressBar percent={usage.memPercent ?? 0} color={METRIC_COLORS.mem} />
                    </div>
                    <span style={value}>
                      {formatBytes(stats.memUsage)}{info?.memoryBytes ? ` / ${formatBytes(info.memoryBytes)}` : ""}
                    </span>
                  </div>
                </div>
              ) : (
                <div style={{ fontSize: 11, color: theme.nodeSubtext, fontStyle: "italic" }}>no agent</div>
              )}
            </>
          );

          if (!onInspect) return <div key={n.id}>{content}</div>;

          return (
            <button
              key={n.id}
              type="button"
              onClick={() => onInspect(n.id)}
              onMouseEnter={() => setHovered(n.id)}
              onMouseLeave={() => setHovered(null)}
              title={`Inspect ${n.name}`}
              style={{
                ...drillButton,
                display: "block",
                width: "calc(100% + 12px)",
                margin: "0 -6px",
                padding: "3px 6px",
                borderRadius: 5,
                background: hovered === n.id ? theme.rowHover : "transparent",
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
