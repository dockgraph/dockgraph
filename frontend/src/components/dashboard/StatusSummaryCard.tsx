import { useMemo, memo } from "react";
import { useTheme } from "../../theme";
import { DashboardCard } from "./DashboardCard";
import type { DGNode } from "../../types";
import { formatUptime } from "../../utils/format";

interface Props {
  nodes: DGNode[];
}

const STATUS_COLORS: Record<string, string> = {
  running: "#22c55e",
  paused: "#f59e0b",
  exited: "#ef4444",
  not_running: "#6b7280",
};

const STATUS_KEYS = ["running", "paused", "exited", "not_running"] as const;

export const StatusSummaryCard = memo(function StatusSummaryCard({ nodes }: Props) {
  const { theme } = useTheme();

  const { statusCounts, networks, volumes, runningContainers } = useMemo(() => {
    const containers = nodes.filter(n => n.type === "container");
    const counts: Record<string, number> = { running: 0, paused: 0, exited: 0, not_running: 0 };
    for (const c of containers) {
      const s = c.status ?? "not_running";
      counts[s] = (counts[s] ?? 0) + 1;
    }
    return {
      statusCounts: counts,
      networks: nodes.filter(n => n.type === "network").length,
      volumes: nodes.filter(n => n.type === "volume").length,
      runningContainers: containers.filter(n => n.status === "running" && n.createdAt),
    };
  }, [nodes]);

  return (
    <DashboardCard title="Status">
      <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
        {STATUS_KEYS.map(status => (
          <div key={status} style={{ display: "flex", alignItems: "center", gap: 8 }}>
            <span style={{ width: 8, height: 8, borderRadius: "50%", background: STATUS_COLORS[status], flexShrink: 0 }} />
            <span style={{ fontSize: 13, color: theme.nodeText, flex: 1 }}>{status.replace("_", " ")}</span>
            <span style={{ fontSize: 14, fontWeight: 600, color: theme.nodeText, fontFamily: "monospace" }}>
              {statusCounts[status] ?? 0}
            </span>
          </div>
        ))}
        <div style={{
          borderTop: `1px solid ${theme.panelBorder}`,
          paddingTop: 8,
          marginTop: 4,
          display: "flex",
          gap: 16,
          fontSize: 12,
          color: theme.nodeSubtext,
        }}>
          <span>{networks} networks</span>
          <span>{volumes} volumes</span>
        </div>
        {runningContainers.length > 0 && (
          <details style={{ fontSize: 11, color: theme.nodeSubtext }}>
            <summary style={{ cursor: "pointer" }}>Created</summary>
            <div style={{ marginTop: 4, display: "flex", flexDirection: "column", gap: 2 }}>
              {runningContainers.map(n => (
                <div key={n.id} style={{ display: "flex", justifyContent: "space-between" }}>
                  <span>{n.name}</span>
                  <span style={{ fontFamily: "monospace" }}>{formatUptime(n.createdAt!)}</span>
                </div>
              ))}
            </div>
          </details>
        )}
      </div>
    </DashboardCard>
  );
});
