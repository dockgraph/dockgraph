/** Semantic status colors shared across all dashboard cards. */
export const STATUS_COLORS = {
  green: "#22c55e",
  red: "#ef4444",
  amber: "#f59e0b",
  blue: "#3b82f6",
  purple: "#8b5cf6",
  gray: "#64748b",
} as const;

/** Metric-specific colors for resource charts and tables. */
export const METRIC_COLORS = {
  cpu: "#64b5f6",
  mem: "#c084fc",
  net: "#4ade80",
  disk: "#fb923c",
} as const;
