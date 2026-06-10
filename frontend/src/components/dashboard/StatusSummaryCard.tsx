import { useMemo, useState, memo } from "react";
import { useTheme } from "../../theme";
import { DashboardCard } from "./DashboardCard";
import { ProgressBar } from "./ProgressBar";
import { STATUS_COLORS } from "./palette";
import type { ResourceTab } from "../table/TableView";
import type { DGNode } from "../../types";

interface Props {
  nodes: DGNode[];
  /** Open the table filtered to the clicked container status. */
  onStatusFilter: (status: string) => void;
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

const STATUS_CONFIG = [
  { key: "running", label: "Running", color: STATUS_COLORS.green },
  { key: "paused", label: "Paused", color: STATUS_COLORS.amber },
  { key: "exited", label: "Exited", color: STATUS_COLORS.red },
  { key: "not_running", label: "Pending", color: STATUS_COLORS.gray },
] as const;

export const StatusSummaryCard = memo(function StatusSummaryCard({ nodes, onStatusFilter, onResourceTab }: Props) {
  const { theme } = useTheme();
  const [hovered, setHovered] = useState<string | null>(null);

  const { counts, networks, volumes, total } = useMemo(() => {
    const containers = nodes.filter(n => n.type === "container");
    const c: Record<string, number> = {};
    for (const n of containers) {
      const status = n.status ?? "not_running";
      c[status] = (c[status] ?? 0) + 1;
    }
    return {
      counts: c,
      total: containers.length,
      networks: nodes.filter(n => n.type === "network").length,
      volumes: nodes.filter(n => n.type === "volume").length,
    };
  }, [nodes]);

  const totalBadge = (
    <span style={{ fontSize: 18, fontWeight: 700, color: theme.nodeText, fontFamily: "var(--dg-font-mono)", lineHeight: 1 }}>
      {total}
    </span>
  );

  return (
    <DashboardCard title="Containers" badge={totalBadge}>
      <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
        {STATUS_CONFIG.map(({ key, label, color }) => {
          const count = counts[key] ?? 0;
          const pct = total > 0 ? (count / total) * 100 : 0;
          return (
            <button
              key={key}
              type="button"
              onClick={() => onStatusFilter(key)}
              onMouseEnter={() => setHovered(key)}
              onMouseLeave={() => setHovered(null)}
              title={`Show ${label.toLowerCase()} containers in the table`}
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
          {[
            { tab: "networks" as const, label: "Networks", count: networks },
            { tab: "volumes" as const, label: "Volumes", count: volumes },
          ].map(r => (
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
