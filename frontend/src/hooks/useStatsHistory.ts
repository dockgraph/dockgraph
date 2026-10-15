import { useMemo } from "react";
import { usePollingFetch } from "./usePollingFetch";

export type TimeRange = "5m" | "1h" | "6h" | "24h";

export interface ContainerTimeSeries {
  cpu: number[];
  mem: number[];
  netRx: number[];
  netTx: number[];
  blockRead: number[];
  blockWrite: number[];
}

export interface StatsHistoryData {
  range: string;
  resolution: number;
  timestamps: number[];
  containers: Record<string, ContainerTimeSeries>;
}

/** Which series the history holds: workloads (default) or per-swarm-node aggregates. */
export type HistoryScope = "workloads" | "nodes";

/**
 * Polls the stats history. `stack` (a compose project / swarm stack name)
 * limits the series to that stack's workloads server-side. `scope` "nodes"
 * returns only the per-swarm-node aggregates (`node:{hostname}`) instead;
 * the backend rejects it combined with a stack, so the stack is dropped then.
 */
export function useStatsHistory(range: TimeRange, stack?: string | null, scope: HistoryScope = "workloads") {
  const url = useMemo(() => {
    if (scope === "nodes") return `/api/stats/history?range=${range}&scope=nodes`;
    return `/api/stats/history?range=${range}${stack ? `&stack=${encodeURIComponent(stack)}` : ''}`;
  }, [range, stack, scope]);
  return usePollingFetch<StatsHistoryData>(url, 10_000);
}
