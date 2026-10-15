import { useMemo, useState, memo } from "react";
import { useTheme } from "../../theme";
import { DashboardCard } from "./DashboardCard";
import { ProgressBar } from "./ProgressBar";
import { STATUS_COLORS } from "./palette";
import type { ResourceTab } from "../table/TableView";
import type { DGNode } from "../../types";

interface Props {
  nodes: DGNode[];
  /**
   * Swarm mode: workloads are service nodes (task containers are not in the
   * graph), so the card summarises services instead of containers.
   */
  swarm?: boolean;
  /** Open the table's `tab` filtered to the clicked status. */
  onStatusFilter: (status: string, tab: ResourceTab) => void;
  /** Open the table on the clicked resource's subtab. */
  onResourceTab: (tab: ResourceTab) => void;
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

const CONTAINER_STATUSES = [
  { key: "running", label: "Running", color: STATUS_COLORS.green },
  { key: "paused", label: "Paused", color: STATUS_COLORS.amber },
  { key: "exited", label: "Exited", color: STATUS_COLORS.red },
  { key: "not_running", label: "Pending", color: STATUS_COLORS.gray },
] as const;

/** Swarm service statuses (backend serviceStatus), plus stack-file services not deployed yet. */
const SERVICE_STATUSES = [
  { key: "running", label: "Running", color: STATUS_COLORS.green },
  { key: "updating", label: "Updating", color: STATUS_COLORS.blue },
  { key: "degraded", label: "Degraded", color: STATUS_COLORS.amber },
  { key: "stopped", label: "Stopped", color: STATUS_COLORS.gray },
  { key: "not_running", label: "Pending", color: STATUS_COLORS.purple },
] as const;

export const StatusSummaryCard = memo(function StatusSummaryCard({ nodes, swarm = false, onStatusFilter, onResourceTab }: Props) {
  const { theme } = useTheme();
  const [hovered, setHovered] = useState<string | null>(null);

  const workloadType = swarm ? "service" : "container";
  const workloadTab: ResourceTab = swarm ? "services" : "containers";
  const noun = swarm ? "services" : "containers";
  const statuses = swarm ? SERVICE_STATUSES : CONTAINER_STATUSES;

  const { counts, containers, networks, volumes, total } = useMemo(() => {
    const workloads = nodes.filter(n => n.type === workloadType);
    const c: Record<string, number> = {};
    for (const n of workloads) {
      const status = n.status ?? "not_running";
      c[status] = (c[status] ?? 0) + 1;
    }
    return {
      counts: c,
      total: workloads.length,
      containers: nodes.filter(n => n.type === "container").length,
      networks: nodes.filter(n => n.type === "network").length,
      volumes: nodes.filter(n => n.type === "volume").length,
    };
  }, [nodes, workloadType]);

  // Swarm mode: standalone containers (not swarm tasks) stay reachable.
  const totals: { tab: ResourceTab; label: string; count: number }[] = [
    ...(swarm && containers > 0 ? [{ tab: "containers" as const, label: "Containers", count: containers }] : []),
    { tab: "networks", label: "Networks", count: networks },
    { tab: "volumes", label: "Volumes", count: volumes },
  ];

  const totalBadge = (
    <span style={{ fontSize: 18, fontWeight: 700, color: theme.nodeText, fontFamily: "var(--dg-font-mono)", lineHeight: 1 }}>
      {total}
    </span>
  );

  return (
    <DashboardCard title={swarm ? "Services" : "Containers"} badge={totalBadge}>
      <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
        {statuses.map(({ key, label, color }) => {
          const count = counts[key] ?? 0;
          const pct = total > 0 ? (count / total) * 100 : 0;
          return (
            <button
              key={key}
              type="button"
              onClick={() => onStatusFilter(key, workloadTab)}
              onMouseEnter={() => setHovered(key)}
              onMouseLeave={() => setHovered(null)}
              title={`Show ${label.toLowerCase()} ${noun} in the table`}
              style={{
                ...drillButton,
                display: "block",
                width: "calc(100% + 12px)",
                margin: "0 -6px",
                padding: "2px 6px",
                borderRadius: 5,
                background: hovered === key ? theme.rowHover : "transparent",
                transition: "background 0.12s",
              }}
            >
              <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 3 }}>
                <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
                  <span style={{ width: 7, height: 7, borderRadius: "50%", background: color, flexShrink: 0 }} />
                  <span style={{ fontSize: 12, color: theme.nodeSubtext }}>{label}</span>
                </div>
                <span style={{ fontSize: 12, fontWeight: 600, color: theme.nodeText, fontFamily: "var(--dg-font-mono)" }}>{count}</span>
              </div>
              <ProgressBar percent={pct} color={color} />
            </button>
          );
        })}

        {/* Resource totals */}
        <div style={{
          display: "flex",
          gap: 12,
          paddingTop: 8,
          borderTop: `1px solid ${theme.panelBorder}`,
          marginTop: 2,
        }}>
          {totals.map(r => (
            <button
              key={r.label}
              type="button"
              onClick={() => onResourceTab(r.tab)}
              onMouseEnter={() => setHovered(r.tab)}
              onMouseLeave={() => setHovered(null)}
              title={`Show ${r.label.toLowerCase()} in the table`}
              style={{
                ...drillButton,
                display: "flex",
                alignItems: "baseline",
                gap: 4,
                padding: "2px 6px",
                margin: "-2px -6px",
                borderRadius: 5,
                background: hovered === r.tab ? theme.rowHover : "transparent",
                transition: "background 0.12s",
              }}
            >
              <span style={{ fontSize: 14, fontWeight: 600, color: theme.nodeText, fontFamily: "var(--dg-font-mono)" }}>{r.count}</span>
              <span style={{ fontSize: 11, color: theme.nodeSubtext }}>{r.label}</span>
            </button>
          ))}
        </div>
      </div>
    </DashboardCard>
  );
});
