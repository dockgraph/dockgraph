const PALETTE = [
  '#3b82f6',
  '#a855f7',
  '#22c55e',
  '#f59e0b',
  '#ef4444',
  '#06b6d4',
  '#ec4899',
  '#f97316',
];

const cache = new Map<string, string>();

export function networkColor(networkName: string): string {
  if (cache.has(networkName)) {
    return cache.get(networkName)!;
  }
  const color = PALETTE[cache.size % PALETTE.length];
  cache.set(networkName, color);
  return color;
}

export const STATUS_COLORS: Record<string, string> = {
  running: '#22c55e',
  unhealthy: '#f59e0b',
  paused: '#3b82f6',
  exited: '#ef4444',
  dead: '#a855f7',
  created: '#06b6d4',
  not_running: '#64748b',
};
