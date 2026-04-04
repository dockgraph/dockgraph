import { useMemo, memo } from "react";
import { useTheme } from "../../theme";
import { DashboardCard } from "./DashboardCard";
import { STATUS_COLORS } from "./palette";
import { evaluateAlerts, type Alert } from "../../utils/alerts";
import type { DGNode } from "../../types";
import type { ContainerStatsData } from "../../types/stats";

interface Props {
  nodes: DGNode[];
  statsMap: Map<string, ContainerStatsData>;
}

const SEVERITY_STYLES: Record<Alert["severity"], { color: string; bg: string; label: string }> = {
  error:   { color: STATUS_COLORS.red,   bg: "rgba(239,68,68,0.08)",  label: "ERR" },
  warning: { color: STATUS_COLORS.amber, bg: "rgba(245,158,11,0.08)", label: "WRN" },
  info:    { color: STATUS_COLORS.blue,  bg: "rgba(59,130,246,0.08)", label: "INF" },
};

export const AlertsCard = memo(function AlertsCard({ nodes, statsMap }: Props) {
  const { theme } = useTheme();
  const alerts = useMemo(() => evaluateAlerts(nodes, statsMap), [nodes, statsMap]);

  const badge = alerts.length > 0 ? (
    <span style={{
      fontSize: 10,
      fontWeight: 600,
      color: SEVERITY_STYLES[alerts[0].severity].color,
      background: SEVERITY_STYLES[alerts[0].severity].bg,
      padding: "2px 6px",
      borderRadius: 4,
      lineHeight: 1,
    }}>
      {alerts.length}
    </span>
  ) : undefined;

  return (
    <DashboardCard title="Alerts" badge={badge} emptyMessage={alerts.length === 0 ? "No issues detected" : undefined}>
      <div style={{ display: "flex", flexDirection: "column", gap: 4, maxHeight: 220, overflowY: "auto" }}>
        {alerts.map((alert, i) => {
          const sev = SEVERITY_STYLES[alert.severity];
          return (
            <div
              key={`${alert.container}-${alert.message}-${i}`}
              style={{
                display: "flex",
                alignItems: "center",
                gap: 8,
                padding: "5px 8px",
                borderRadius: 4,
                background: sev.bg,
              }}
            >
              <span style={{
                fontSize: 9,
                fontWeight: 700,
                color: sev.color,
                letterSpacing: "0.04em",
                flexShrink: 0,
                width: 24,
              }}>
                {sev.label}
              </span>
              <span style={{
                fontSize: 12,
                color: theme.nodeText,
                fontWeight: 500,
                flexShrink: 0,
              }}>
                {alert.container}
              </span>
              <span style={{ fontSize: 11, color: theme.nodeSubtext, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                {alert.message}
              </span>
            </div>
          );
        })}
      </div>
    </DashboardCard>
  );
});
