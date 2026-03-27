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

const MAX_CACHE_SIZE = 256;
const cache = new Map<string, string>();

function hashString(str: string): number {
  let hash = 0;
  for (let i = 0; i < str.length; i++) {
    hash = ((hash << 5) - hash + str.charCodeAt(i)) | 0;
  }
  return Math.abs(hash);
}

export function networkColor(networkName: string): string {
  if (cache.has(networkName)) {
    return cache.get(networkName)!;
  }
  if (cache.size >= MAX_CACHE_SIZE) {
    const oldest = cache.keys().next().value!;
    cache.delete(oldest);
  }
  const color = PALETTE[hashString(networkName) % PALETTE.length];
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

export const STATUS_LABELS: Record<string, string> = {
  running: 'Running',
  unhealthy: 'Unhealthy',
  paused: 'Paused',
  exited: 'Exited',
  dead: 'Dead',
  created: 'Created',
  not_running: 'Not running',
};
