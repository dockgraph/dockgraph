import { useMemo, memo } from "react";
import { useTheme } from "../../theme";
import { DashboardCard } from "./DashboardCard";
import { ProgressBar } from "./ProgressBar";
import { STATUS_COLORS } from "./palette";
import type { DGNode } from "../../types";

interface Props {
  nodes: DGNode[];
}

const STATUS_CONFIG = [
  { key: "running", label: "Running", color: STATUS_COLORS.green },
  { key: "paused", label: "Paused", color: STATUS_COLORS.amber },
  { key: "exited", label: "Exited", color: STATUS_COLORS.red },
  { key: "not_running", label: "Pending", color: STATUS_COLORS.gray },
] as const;

export const StatusSummaryCard = memo(function StatusSummaryCard({ nodes }: Props) {
  const { theme } = useTheme();

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
    <span style={{ fontSize: 18, fontWeight: 700, color: theme.nodeText, fontFamily: "monospace", lineHeight: 1 }}>
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
            <div key={key}>
              <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 3 }}>
                <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
                  <span style={{ width: 7, height: 7, borderRadius: "50%", background: color, flexShrink: 0 }} />
                  <span style={{ fontSize: 12, color: theme.nodeSubtext }}>{label}</span>
                </div>
                <span style={{ fontSize: 12, fontWeight: 600, color: theme.nodeText, fontFamily: "monospace" }}>{count}</span>
              </div>
              <ProgressBar percent={pct} color={color} />
            </div>
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
            { label: "Networks", count: networks },
            { label: "Volumes", count: volumes },
          ].map(r => (
            <div key={r.label} style={{ display: "flex", alignItems: "baseline", gap: 4 }}>
              <span style={{ fontSize: 14, fontWeight: 600, color: theme.nodeText, fontFamily: "monospace" }}>{r.count}</span>
              <span style={{ fontSize: 11, color: theme.nodeSubtext }}>{r.label}</span>
            </div>
          ))}
        </div>
      </div>
    </DashboardCard>
  );
});
