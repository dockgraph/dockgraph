import { useMemo, memo } from "react";
import { useTheme } from "../../theme";
import { DashboardCard } from "./DashboardCard";
import { evaluateAlerts, type Alert } from "../../utils/alerts";
import type { DGNode } from "../../types";
import type { ContainerStatsData } from "../../types/stats";

interface Props {
  nodes: DGNode[];
  statsMap: Map<string, ContainerStatsData>;
}

const SEVERITY_COLORS: Record<Alert["severity"], string> = {
  error: "#ef4444",
  warning: "#f59e0b",
  info: "#3b82f6",
};

const SEVERITY_ICONS: Record<Alert["severity"], string> = {
  error: "\u25cf",
  warning: "\u25b2",
  info: "\u25cb",
};

export const AlertsCard = memo(function AlertsCard({ nodes, statsMap }: Props) {
  const { theme } = useTheme();
  const alerts = useMemo(() => evaluateAlerts(nodes, statsMap), [nodes, statsMap]);

  return (
    <DashboardCard title="Alerts">
      <div style={{ display: "flex", flexDirection: "column", gap: 6, maxHeight: 240, overflowY: "auto" }}>
        {alerts.length === 0 && (
          <span style={{ fontSize: 12, color: theme.nodeSubtext, padding: 8 }}>
            No issues detected
          </span>
        )}
        {alerts.map((alert, i) => (
          <div key={`${alert.container}-${alert.message}-${i}`} style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 12 }}>
            <span style={{ color: SEVERITY_COLORS[alert.severity], flexShrink: 0 }}>
              {SEVERITY_ICONS[alert.severity]}
            </span>
            <span style={{ color: theme.nodeText, fontWeight: 500 }}>{alert.container}</span>
            <span style={{ color: theme.nodeSubtext }}>{alert.message}</span>
          </div>
        ))}
      </div>
    </DashboardCard>
  );
});
