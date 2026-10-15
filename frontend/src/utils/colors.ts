import { stripStackPrefix } from './stack';
import { STATS_CPU_WARN, STATS_CPU_CRIT, STATS_THROTTLE_CRIT } from './constants';

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

/** Identity color for volumes — shared by volume nodes and volume_mount edges. */
export const VOLUME_COLOR = '#f97316';

const MAX_CACHE_SIZE = 256;
const cache = new Map<string, string>();

export function hashString(str: string): number {
  let hash = 0;
  for (let i = 0; i < str.length; i++) {
    hash = ((hash << 5) - hash + str.charCodeAt(i)) | 0;
  }
  return Math.abs(hash);
}

/**
 * Deterministic identity colour for a network name. When the owning stack is
 * given, its `{stack}_` prefix is dropped before hashing so the same logical
 * network (e.g. `shop_backend` and `blog_backend`) keeps one colour across
 * stacks.
 */
export function networkColor(name: string, stack?: string): string {
  const networkName = stripStackPrefix(name, stack);
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

/** Neutral colour for workloads that belong to no stack. */
export const NO_STACK_COLOR = '#64748b';

/**
 * Deterministic identity colour for a stack/project, hashed like network
 * colours. Workloads without a stack get a neutral slate.
 */
export function stackColor(stack: string | undefined): string {
  if (!stack) return NO_STACK_COLOR;
  return PALETTE[hashString(stack) % PALETTE.length];
}

/** Colours for swarm node states ("ready", "down", ...). */
export const SWARM_NODE_STATE_COLORS: Record<string, string> = {
  ready: '#22c55e',
  down: '#ef4444',
  disconnected: '#ef4444',
  unknown: '#f59e0b',
};

/** Colour of a swarm node state dot; unknown states read amber. */
export function swarmNodeStateColor(state: string | undefined): string {
  return SWARM_NODE_STATE_COLORS[state ?? 'unknown'] ?? SWARM_NODE_STATE_COLORS.unknown;
}

/**
 * Identity colours of swarm node roles in the per-node view (role group
 * frame, title tab, node box accent, role badge, minimap). Violet for
 * managers and cyan for workers stay clear of the green/amber/red state
 * colours; the light variants are darker for contrast on the paper canvas.
 */
export const SWARM_ROLE_COLORS: Record<'manager' | 'worker', { dark: string; light: string }> = {
  manager: { dark: '#a78bfa', light: '#7c3aed' },
  worker: { dark: '#22d3ee', light: '#0e7490' },
};

/** Colour of a swarm node role for the given theme mode. */
export function swarmRoleColor(role: 'manager' | 'worker', mode: 'dark' | 'light'): string {
  return SWARM_ROLE_COLORS[role][mode];
}

export const STATUS_COLORS: Record<string, string> = {
  running: '#22c55e',
  unhealthy: '#f59e0b',
  paused: '#3b82f6',
  exited: '#ef4444',
  dead: '#a855f7',
  created: '#06b6d4',
  not_running: '#64748b',
  // Swarm service states.
  degraded: '#f59e0b',
  updating: '#3b82f6',
  stopped: '#ef4444',
  unknown: '#64748b',
};

/** Returns a semantic color for CPU usage: green (ok), amber (warn), red (critical). */
export function cpuColor(cpu: number, throttle: number): string {
  if (cpu >= STATS_CPU_CRIT || throttle >= STATS_THROTTLE_CRIT) return '#ef4444';
  if (cpu >= STATS_CPU_WARN || throttle > 0) return '#f59e0b';
  return '#22c55e';
}

export const STATUS_LABELS: Record<string, string> = {
  running: 'Running',
  unhealthy: 'Unhealthy',
  paused: 'Paused',
  exited: 'Exited',
  dead: 'Dead',
  created: 'Created',
  not_running: 'Not running',
  degraded: 'Degraded',
  updating: 'Updating',
  stopped: 'Stopped',
  unknown: 'Unknown',
};
