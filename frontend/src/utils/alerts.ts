import type { DGNode } from "../types";
import type { ContainerStatsData } from "../types/stats";

export interface Alert {
  severity: "error" | "warning" | "info";
  /** Graph node the alert refers to, for the detail panel. */
  nodeId: string;
  /** Workload (container or swarm service) name. */
  container: string;
  message: string;
}

/**
 * Alerts for the workloads in `nodes`: containers, and swarm services (whose
 * stats entry is the aggregate of their tasks, keyed by service name).
 */
export function evaluateAlerts(
  nodes: DGNode[],
  stats: Map<string, ContainerStatsData>,
): Alert[] {
  const alerts: Alert[] = [];

  for (const node of nodes) {
    if (node.type !== "container" && node.type !== "service") continue;
    const push = (severity: Alert["severity"], message: string) =>
      alerts.push({ severity, nodeId: node.id, container: node.name, message });

    // A service aggregate sums its tasks' CPU, so compare the per-replica
    // average with the per-container threshold.
    let replicas = 1;
    if (node.type === "container") {
      if (node.status === "exited") push("error", "Container exited");
      if (node.status === "restarting") push("warning", "Container restarting");
    } else {
      const r = node.service?.replicas;
      if (node.status === "degraded") {
        push("warning", r ? `Service degraded: ${r.running}/${r.desired} running` : "Service degraded");
      }
      replicas = Math.max(1, r?.running ?? 1);
    }

    const s = stats.get(node.name);
    if (!s) continue;

    const cpu = s.cpuPercent / replicas;
    if (cpu > 80) {
      push("warning", `High CPU: ${cpu.toFixed(1)}%${replicas > 1 ? " per replica" : ""}`);
    }
    if (s.memLimit > 0 && s.memUsage / s.memLimit > 0.9) {
      push("warning", "Memory usage > 90% of limit");
    }
    if (s.netRxErrors + s.netTxErrors > 0) {
      push("info", `Network errors: ${s.netRxErrors + s.netTxErrors}`);
    }
  }

  const order = { error: 0, warning: 1, info: 2 };
  alerts.sort((a, b) => order[a.severity] - order[b.severity]);
  return alerts;
}
