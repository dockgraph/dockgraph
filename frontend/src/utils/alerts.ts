import type { DGNode } from "../types";
import type { ContainerStatsData } from "../types/stats";

export interface Alert {
  severity: "error" | "warning" | "info";
  container: string;
  message: string;
}

export function evaluateAlerts(
  nodes: DGNode[],
  stats: Map<string, ContainerStatsData>,
): Alert[] {
  const alerts: Alert[] = [];

  for (const node of nodes) {
    if (node.type !== "container") continue;

    if (node.status === "exited") {
      alerts.push({ severity: "error", container: node.name, message: "Container exited" });
    }
    if (node.status === "restarting") {
      alerts.push({ severity: "warning", container: node.name, message: "Container restarting" });
    }

    const s = stats.get(node.name);
    if (!s) continue;

    if (s.cpuPercent > 80) {
      alerts.push({ severity: "warning", container: node.name, message: `High CPU: ${s.cpuPercent.toFixed(1)}%` });
    }
    if (s.memLimit > 0 && s.memUsage / s.memLimit > 0.9) {
      alerts.push({ severity: "warning", container: node.name, message: "Memory usage > 90% of limit" });
    }
    if (s.netRxErrors + s.netTxErrors > 0) {
      alerts.push({ severity: "info", container: node.name, message: `Network errors: ${s.netRxErrors + s.netTxErrors}` });
    }
  }

  const order = { error: 0, warning: 1, info: 2 };
  alerts.sort((a, b) => order[a.severity] - order[b.severity]);
  return alerts;
}
