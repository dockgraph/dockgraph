import { memo } from "react";
import { useTheme } from "../../theme";
import { useRowHover } from "../../hooks/useRowHover";
import { STATUS_COLORS, networkColor } from "../../utils/colors";
import { formatBytes } from "../../utils/formatBytes";
import { INACTIVE_OPACITY } from "../../utils/constants";
import { tableRow } from "./tableStyles";
import type { DGNode } from "../../types";
import type { ContainerStatsData } from "../../types/stats";

interface Props {
  node: DGNode;
  stats: ContainerStatsData | undefined;
  selected: boolean;
  onClick: (nodeId: string) => void;
  gridTemplate: string;
}

const ACTIVE_STATUSES = new Set(["running", "unhealthy"]);

function formatPorts(node: DGNode): string {
  if (!node.ports || node.ports.length === 0) return "\u2014";
  const visible = node.ports.slice(0, 3);
  const text = visible.map((p) => `${p.host}:${p.container}`).join(", ");
  const overflow = node.ports.length - 3;
  return overflow > 0 ? `${text} +${overflow}` : text;
}

function formatNetwork(node: DGNode): string {
  if (!node.networkId) return "\u2014";
  return node.networkId.replace(/^network:/, "");
}

export const ContainerRow = memo(function ContainerRow({
  node,
  stats,
  selected,
  onClick,
  gridTemplate,
}: Props) {
  const { theme } = useTheme();
  const styles = tableRow(theme);
  const { handlers, rowStyle } = useRowHover();
  const isActive = ACTIVE_STATUSES.has(node.status ?? "");
  const isGhost = node.status === "not_running";
  const opacity = isActive ? 1 : INACTIVE_OPACITY;
  const statusColor = STATUS_COLORS[node.status ?? ""] ?? STATUS_COLORS.created;

  return (
    <div
      style={{ opacity }}
      {...handlers}
    >
      <div
        role="button"
        tabIndex={0}
        onClick={() => onClick(node.id)}
        onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onClick(node.id); } }}
        style={rowStyle(theme, selected, {
          ...styles.row,
          gridTemplateColumns: gridTemplate,
          ...(isGhost ? { borderBottomStyle: "dashed" as const } : {}),
        })}
      >
        <span style={{ fontWeight: 600, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }} title={node.name}>
          {node.name}
        </span>
        <span style={{ color: theme.nodeSubtext, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }} title={node.image}>
          {node.image ?? "\u2014"}
        </span>
        <span style={{ display: "flex", alignItems: "center", gap: 6 }}>
          <span
            data-testid="status-dot"
            style={{ width: 8, height: 8, borderRadius: "50%", background: statusColor, flexShrink: 0 }}
          />
          {node.status ?? "unknown"}
        </span>
        <span style={{ color: theme.nodeSubtext, fontFamily: "monospace", fontSize: 11, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
          {formatPorts(node)}
        </span>
        <span style={{ color: node.networkId ? networkColor(formatNetwork(node)) : theme.nodeSubtext, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
          {formatNetwork(node)}
        </span>
      </div>
      <div style={styles.statsRow}>
        {stats ? (
          <>
            <span>
              CPU{" "}
              <span style={styles.statsValue("#64b5f6")}>
                {stats.cpuPercent.toFixed(1)}%
              </span>
            </span>
            <span>
              MEM{" "}
              <span style={styles.statsValue("#c084fc")}>
                {formatBytes(stats.memUsage)} / {formatBytes(stats.memLimit)}
              </span>
            </span>
            <span>
              NET{" "}
              <span style={styles.statsValue("#4ade80")}>
                &#8595;{formatBytes(stats.netRx)}
              </span>{" "}
              <span style={styles.statsValue("#f97316")}>
                &#8593;{formatBytes(stats.netTx)}
              </span>
            </span>
          </>
        ) : (
          <span style={{ opacity: 0.4 }}>No stats available</span>
        )}
      </div>
    </div>
  );
});
