import type { DGNode, DGEdge, TaskInfo } from '../types';
import type { ContainerStatsData } from '../types/stats';

/** Label Docker Compose sets on every resource of a project. */
export const COMPOSE_PROJECT_LABEL = 'com.docker.compose.project';
/** Label `docker stack deploy` sets on every resource of a swarm stack. */
export const STACK_NAMESPACE_LABEL = 'com.docker.stack.namespace';

/**
 * Scope key for workloads that belong to no compose project or stack. The
 * leading underscore can't start a real project/stack name, so it never
 * collides with one.
 */
export const STANDALONE_STACK = '_standalone';

/**
 * Returns the compose project or swarm stack a node belongs to, or undefined
 * when it has none. The backend-resolved `stack` wins; the labels are a
 * fallback for payloads that predate it.
 */
export function projectOf(node: DGNode): string | undefined {
  return (
    node.stack ||
    node.labels?.[COMPOSE_PROJECT_LABEL] ||
    node.labels?.[STACK_NAMESPACE_LABEL] ||
    undefined
  );
}

/**
 * True for nodes that run code (containers and swarm services). Swarm nodes
 * (cluster machines) are infrastructure, not workloads.
 */
export function isWorkload(node: DGNode): boolean {
  return node.type === 'container' || node.type === 'service';
}

/**
 * True when the dashboard should summarise swarm services rather than
 * containers. The scoped workloads decide: any service means services
 * (before the system info has loaded too), and containers only, e.g. a
 * Compose project on a swarm-mode daemon, mean containers. Only a scope
 * with no workload falls back to the mode: an empty named stack in swarm
 * mode is a swarm stack, while the standalone scope only ever holds
 * project-less containers.
 */
export function showsSwarmWorkloads(nodes: DGNode[], swarm: boolean, stack: string | null | undefined): boolean {
  if (nodes.some((n) => n.type === 'service')) return true;
  if (nodes.some((n) => n.type === 'container')) return false;
  return swarm && stack !== STANDALONE_STACK;
}

/**
 * True while a swarm service has at least one replica running. Its status
 * can't tell: degraded covers 0/N, and updating wins even with nothing
 * running. Without replica counts, only a running status counts.
 */
export function isServiceLive(node: DGNode): boolean {
  const running = node.service?.replicas?.running;
  return running !== undefined ? running > 0 : node.status === 'running';
}

/** Prefix of the per-swarm-node aggregate keys in the live stats map and history. */
export const NODE_STATS_PREFIX = 'node:';

/** Stats key of a swarm node's aggregate: `node:{hostname}`. */
export function nodeStatsKey(hostname: string): string {
  return `${NODE_STATS_PREFIX}${hostname}`;
}

/**
 * True for per-swarm-node aggregate stats keys (`node:{hostname}`). Docker
 * names can't contain a colon, so these never collide with a workload.
 */
export function isNodeStatsKey(key: string): boolean {
  return key.startsWith(NODE_STATS_PREFIX);
}

/** Chart/legend label of a stats series: node aggregates show the bare hostname. */
export function seriesLabel(key: string): string {
  return isNodeStatsKey(key) ? key.slice(NODE_STATS_PREFIX.length) : key;
}

/**
 * Swarm task container names are `{service}.{slot}.{taskId}` (replicated) or
 * `{service}.{nodeId}.{taskId}` (global); swarm IDs are 25 lowercase base-36
 * characters. Service names may contain dots, so the service is everything
 * before the final two task segments. Mirrors the backend's task-name check in
 * stats history; every frontend task-name check goes through serviceOfTask.
 */
const TASK_NAME = /^(.+)\.(?:\d+|[a-z0-9]{25})\.[a-z0-9]{25}$/;

/** Service a swarm task container name belongs to, or undefined for any other name. */
export function serviceOfTask(name: string): string | undefined {
  return TASK_NAME.exec(name)?.[1];
}

/**
 * True for a swarm task series whose service aggregate is also present
 * (`has(service)`): the aggregate already sums it, so showing both counts the
 * service twice. Tasks of a service without an aggregate are not covered, so
 * their usage is never lost.
 */
export function isCoveredTask(key: string, has: (key: string) => boolean): boolean {
  const svc = serviceOfTask(key);
  return svc !== undefined && has(svc);
}

/**
 * Workload entries of the live stats map, each counted once: drops per-node
 * aggregates, and swarm task series covered by their service aggregate (see
 * isCoveredTask).
 */
export function consumerStats(stats: Map<string, ContainerStatsData>): Map<string, ContainerStatsData> {
  const out = new Map<string, ContainerStatsData>();
  for (const [key, value] of stats) {
    if (isNodeStatsKey(key) || isCoveredTask(key, (k) => stats.has(k))) continue;
    out.set(key, value);
  }
  return out;
}

/**
 * History series keyed by workload, each workload once: drops the swarm task
 * series covered by their service aggregate (see isCoveredTask), as
 * consumerStats does for live stats.
 */
export function withoutCoveredTasks<T>(series: Record<string, T>): Record<string, T> {
  const has = (key: string) => Object.hasOwn(series, key);
  return Object.fromEntries(Object.entries(series).filter(([key]) => !isCoveredTask(key, has)));
}

/** Returns the stats map without the per-node aggregates (workload entries only). */
export function withoutNodeStats(stats: Map<string, ContainerStatsData>): Map<string, ContainerStatsData> {
  let hasNode = false;
  for (const key of stats.keys()) {
    if (isNodeStatsKey(key)) { hasNode = true; break; }
  }
  if (!hasNode) return stats;
  const out = new Map<string, ContainerStatsData>();
  for (const [key, value] of stats) {
    if (!isNodeStatsKey(key)) out.set(key, value);
  }
  return out;
}

export interface StackSummary {
  /** Stack/project name, or STANDALONE_STACK for workloads with no project. */
  name: string;
  running: number;
  total: number;
}

/**
 * Summarises every stack/project present in the graph with its running and
 * total workload counts, sorted by name. Workloads with no project are
 * grouped under STANDALONE_STACK, listed last.
 */
export function listStacks(nodes: DGNode[]): StackSummary[] {
  const groups = new Map<string, StackSummary>();
  for (const n of nodes) {
    if (!isWorkload(n)) continue;
    const name = projectOf(n) ?? STANDALONE_STACK;
    const group = groups.get(name) ?? { name, running: 0, total: 0 };
    group.total++;
    if (n.status === 'running') group.running++;
    groups.set(name, group);
  }
  return Array.from(groups.values()).sort((a, b) => {
    if (a.name === STANDALONE_STACK) return 1;
    if (b.name === STANDALONE_STACK) return -1;
    return a.name.localeCompare(b.name);
  });
}

/** True when a node belongs to the given scope (a stack name or STANDALONE_STACK). */
export function inStack(node: DGNode, stack: string): boolean {
  const project = projectOf(node);
  return stack === STANDALONE_STACK ? project === undefined : project === stack;
}

/**
 * Narrows the graph to one stack. Keeps the stack's own nodes, plus any
 * network or volume a kept workload references (e.g. an external network),
 * plus the edges whose ends both survive. Swarm nodes are always kept, so
 * the per-node view still shows the machines a stack has no task on. A null
 * stack returns the input.
 *
 * For the standalone scope only workloads without a project are kept as
 * "own" nodes; networks and volumes come in only when referenced, so
 * project-owned resources don't leak into it.
 */
export function filterGraphByStack(
  nodes: DGNode[],
  edges: DGEdge[],
  stack: string | null,
): { nodes: DGNode[]; edges: DGEdge[] } {
  if (!stack) return { nodes, edges };

  const kept = new Set<string>();
  for (const n of nodes) {
    if (n.type === 'swarmnode') {
      kept.add(n.id);
    } else if (isWorkload(n) ? inStack(n, stack) : stack !== STANDALONE_STACK && inStack(n, stack)) {
      kept.add(n.id);
    }
  }

  // Pull in the networks and volumes referenced by kept workloads.
  const workloadIds = new Set(nodes.filter((n) => isWorkload(n) && kept.has(n.id)).map((n) => n.id));
  for (const n of nodes) {
    if (workloadIds.has(n.id) && n.networkId) kept.add(n.networkId);
  }
  for (const e of edges) {
    if (e.type === 'secondary_network' && workloadIds.has(e.source)) kept.add(e.target);
    if (e.type === 'volume_mount' && workloadIds.has(e.target)) kept.add(e.source);
  }

  const scopedNodes = nodes.filter((n) => kept.has(n.id));
  const present = new Set(scopedNodes.map((n) => n.id));
  const scopedEdges = edges.filter((e) => present.has(e.source) && present.has(e.target));
  return { nodes: scopedNodes, edges: scopedEdges };
}

/**
 * Strips the `{stack}_` prefix from a resource name so that the same logical
 * network in different stacks (e.g. `shop_backend`, `blog_backend`) hashes to
 * the same colour. Names without a known prefix are returned unchanged.
 */
export function stripStackPrefix(name: string, stack: string | undefined): string {
  if (stack && name.startsWith(`${stack}_`) && name.length > stack.length + 1) {
    return name.slice(stack.length + 1);
  }
  return name;
}

/**
 * Resolves a cross-reference (`type:name`) to a graph node ID. Docker and
 * compose often refer to workloads by short name (`web`) while graph nodes
 * carry the prefixed form (`shop_web` for a swarm service, `shop-web-1` for a
 * compose container, `shop_web.1.<taskid>` for a task container). Candidates
 * in `contextStack` win over same-named workloads in other stacks, and a
 * container reference may resolve to a service (and vice versa). Returns
 * `targetId` unchanged when nothing matches.
 */
export function resolveNodeRef(nodes: DGNode[], targetId: string, contextStack?: string): string {
  if (nodes.some((n) => n.id === targetId)) return targetId;
  const sepIdx = targetId.indexOf(':');
  if (sepIdx < 0) return targetId;
  const type = targetId.slice(0, sepIdx);
  const name = targetId.slice(sepIdx + 1);
  const baseName = serviceOfTask(name) ?? name;

  const types = type === 'container' ? ['container', 'service']
    : type === 'service' ? ['service', 'container']
    : [type];

  let best: { id: string; score: number } | null = null;
  for (const n of nodes) {
    const typeRank = types.indexOf(n.type);
    if (typeRank < 0) continue;

    let score = 0;
    if (n.name === name) score = 400;
    else if (n.name === baseName) score = 350;
    else if (contextStack && (n.name === `${contextStack}_${name}` || n.name.startsWith(`${contextStack}-${name}-`))) score = 300;
    else if (n.name.endsWith(`_${name}`) || n.name.endsWith(`-${name}`)) {
      score = contextStack && projectOf(n) === contextStack ? 200 : 100;
    }
    if (score === 0) continue;
    score -= typeRank;
    if (!best || score > best.score) best = { id: n.id, score };
  }
  return best ? best.id : targetId;
}

/**
 * The container name Docker gives a task: `{service}.{slot}.{taskId}` for
 * replicated services and `{service}.{nodeId}.{taskId}` for global ones.
 * Per-task stats are keyed by it.
 */
export function taskContainerName(service: string, task: TaskInfo): string {
  return `${service}.${task.slot ? task.slot : task.nodeId ?? ''}.${task.id}`;
}
