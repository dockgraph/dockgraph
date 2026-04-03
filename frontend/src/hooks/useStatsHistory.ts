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

export function useStatsHistory(range: TimeRange) {
  const url = useMemo(() => `/api/stats/history?range=${range}`, [range]);
  return usePollingFetch<StatsHistoryData>(url, 10_000);
}
