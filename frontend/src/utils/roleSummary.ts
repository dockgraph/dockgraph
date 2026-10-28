import { listSwarmNodes, swarmRole, nodeUsage, clampPercent, type NodeUsage } from './nodeTransform';
import { nodeStatsKey } from './stack';
import type { DGNode, SwarmRole } from '../types';
import type { ContainerStatsData, SwarmManager } from '../types/stats';

/** One swarm node of a role group, as the group's panel lists it. */
export interface RoleMember {
  node: DGNode;
  /** Tasks its box shows (see boxTaskCounts). */
  tasks: number;
  /** Aggregate stats, when the node's agent reports them. */
  stats?: ContainerStatsData;
  usage: NodeUsage;
}

/** Counts per distinct value, most common first. */
export type Tally = { value: string; count: number }[];

/** Totals of the Managers or Workers group of the per-node view. */
export interface RoleSummary {
  /** In display order: leader first, then by hostname. */
  members: RoleMember[];
  tasks: number;
  states: Tally;
  availability: Tally;
  engines: Tally;
  /** Capacity of all the group's nodes. */
  cores: number;
  memoryBytes: number;
  /**
   * Usage relative to the capacity of the nodes whose agent reports stats,
   * so nodes without an agent don't read as idle. Undefined when none does.
   */
  usage?: {
    reporting: number;
    /** CPU capacity of the reporting nodes. */
    cores: number;
    cpuPercent?: number;
    memPercent?: number;
    memUsed: number;
    memCapacity: number;
  };
}

function tally(values: string[]): Tally {
  const counts = new Map<string, number>();
  for (const v of values) counts.set(v, (counts.get(v) ?? 0) + 1);
  return [...counts]
    .map(([value, count]) => ({ value, count }))
    .sort((a, b) => b.count - a.count || a.value.localeCompare(b.value));
}

/**
 * Summarizes the swarm nodes of one role for its group panel. `taskCounts`
 * are the boxes' own (boxTaskCounts), so the totals add up to the group's
 * count; they are passed in because they only change with the graph, while
 * this runs on every stats update.
 */
export function summarizeRole(
  dgNodes: DGNode[],
  role: SwarmRole,
  statsMap: Map<string, ContainerStatsData>,
  taskCounts: Map<string, number>,
): RoleSummary {
  const members: RoleMember[] = listSwarmNodes(dgNodes)
    .filter((n) => swarmRole(n) === role)
    .map((node) => {
      const stats = statsMap.get(nodeStatsKey(node.name));
      return { node, tasks: taskCounts.get(node.id) ?? 0, stats, usage: nodeUsage(node.swarmNode, stats) };
    });

  let cores = 0;
  let memoryBytes = 0;
  // Docker counts 100% CPU per core, so the share of the capacity is the
  // summed percentages over the summed cores.
  let cpuPercentSum = 0;
  let reportingCores = 0;
  let memUsed = 0;
  let memCapacity = 0;
  let reporting = 0;
  for (const { node, stats, usage } of members) {
    const info = node.swarmNode;
    cores += usage.cores ?? 0;
    memoryBytes += info?.memoryBytes ?? 0;
    if (!stats) continue;
    reporting++;
    if (usage.cores) {
      cpuPercentSum += stats.cpuPercent;
      reportingCores += usage.cores;
    }
    if (info?.memoryBytes) {
      memUsed += stats.memUsage;
      memCapacity += info.memoryBytes;
    }
  }

  return {
    members,
    tasks: members.reduce((n, m) => n + m.tasks, 0),
    states: tally(members.map((m) => m.node.swarmNode?.state ?? m.node.status ?? 'unknown')),
    availability: tally(members.map((m) => m.node.swarmNode?.availability ?? 'unknown')),
    engines: tally(members.map((m) => m.node.swarmNode?.engineVersion ?? 'unknown')),
    cores,
    memoryBytes,
    usage: reporting === 0 ? undefined : {
      reporting,
      cores: reportingCores,
      cpuPercent: reportingCores ? clampPercent(cpuPercentSum / reportingCores) : undefined,
      memPercent: memCapacity ? clampPercent((memUsed / memCapacity) * 100) : undefined,
      memUsed,
      memCapacity,
    },
  };
}

/** Health of the managers' raft consensus. */
export interface QuorumHealth {
  managers: number;
  reachable: number;
  /** Managers that must be reachable for the swarm to act: a majority. */
  quorum: number;
  /** How many more managers can be lost; negative once quorum is lost. */
  spare: number;
  leader?: string;
  /** Managers not reachable (unreachable or unknown). */
  unreachable: SwarmManager[];
}

/** Quorum arithmetic of the swarm's managers: a majority of them must be reachable. */
export function quorumHealth(managers: SwarmManager[]): QuorumHealth {
  const reachable = managers.filter((m) => m.reachability === 'reachable').length;
  const quorum = Math.floor(managers.length / 2) + 1;
  return {
    managers: managers.length,
    reachable,
    quorum,
    spare: reachable - quorum,
    leader: managers.find((m) => m.leader)?.name,
    unreachable: managers.filter((m) => m.reachability !== 'reachable'),
  };
}
