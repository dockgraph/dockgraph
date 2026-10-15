import type { Node as RFNode } from '@xyflow/react';
import { projectOf } from './stack';
import type { DGNode, TaskInfo, SwarmNodeGroupData, NodeServiceCardData, RoleGroupData, ControlSummaryData, SwarmRole } from '../types';

/** Id of the control-plane summary badge between the Managers and Workers groups. */
export const CONTROL_SUMMARY_ID = 'controlsummary';

/** Group id for tasks the scheduler hasn't placed on a node yet. */
export const UNASSIGNED_NODE_GROUP_ID = 'nodegroup:unassigned';

/** A running-desired swarm task together with the service it belongs to. */
export interface PlacedTask {
  service: DGNode;
  task: TaskInfo;
}

/** Leader first, then the other managers, then workers; by hostname within each. */
function compareSwarmNodes(a: DGNode, b: DGNode): number {
  const rank = (n: DGNode) => (n.swarmNode?.leader ? 0 : n.swarmNode?.role === 'manager' ? 1 : 2);
  return rank(a) - rank(b) || a.name.localeCompare(b.name);
}

/** Stack (no-stack last), then service name, then slot, then task id. */
function comparePlacedTasks(a: PlacedTask, b: PlacedTask): number {
  const sa = projectOf(a.service);
  const sb = projectOf(b.service);
  if (sa !== sb) {
    if (sa === undefined) return 1;
    if (sb === undefined) return -1;
    return sa.localeCompare(sb);
  }
  return (
    a.service.name.localeCompare(b.service.name) ||
    (a.task.slot ?? 0) - (b.task.slot ?? 0) ||
    (a.task.nodeId ?? '').localeCompare(b.task.nodeId ?? '') ||
    a.task.id.localeCompare(b.task.id)
  );
}

/** The swarm nodes in the graph, in display order (leader, managers, workers). */
export function listSwarmNodes(dgNodes: DGNode[]): DGNode[] {
  return dgNodes.filter((n) => n.type === 'swarmnode').sort(compareSwarmNodes);
}

/**
 * True for a task that is current work on its node: any desired state except
 * "shutdown" and "remove", unless the task has finished (state "complete").
 * Job tasks carry desired state "complete" while they still run, so those
 * stay listed. Mirrors the backend's active-task check in buildServiceInfo;
 * the other tasks it sends are recent completions and failures, shown only
 * in the service detail panel.
 */
export function isActiveTask(task: TaskInfo): boolean {
  return task.desiredState !== 'shutdown' && task.desiredState !== 'remove' && task.state !== 'complete';
}

/**
 * Places every active task (see isActiveTask) of the services in `dgNodes`
 * onto its swarm node. Keys are swarmnode graph ids, or
 * UNASSIGNED_NODE_GROUP_ID for tasks without a (known) node — typically
 * pending tasks the scheduler couldn't place. Each list is sorted by stack,
 * service and slot, so tasks read as grouped by service.
 */
export function placeTasks(dgNodes: DGNode[]): Map<string, PlacedTask[]> {
  const byHostname = new Map<string, string>();
  const byNodeId = new Map<string, string>();
  for (const n of dgNodes) {
    if (n.type !== 'swarmnode') continue;
    byHostname.set(n.name, n.id);
    if (n.swarmNode?.id) byNodeId.set(n.swarmNode.id, n.id);
  }

  const placed = new Map<string, PlacedTask[]>();
  for (const service of dgNodes) {
    if (service.type !== 'service') continue;
    for (const task of service.service?.tasks ?? []) {
      if (!isActiveTask(task)) continue;
      const group =
        (task.nodeHostname && byHostname.get(task.nodeHostname)) ||
        (task.nodeId && byNodeId.get(task.nodeId)) ||
        UNASSIGNED_NODE_GROUP_ID;
      const list = placed.get(group) ?? [];
      list.push({ service, task });
      placed.set(group, list);
    }
  }
  for (const list of placed.values()) list.sort(comparePlacedTasks);
  return placed;
}

/** Label of a task card: `{service}.{slot}`, or the bare service name for global tasks. */
export function taskLabel(serviceName: string, task: TaskInfo): string {
  return task.slot ? `${serviceName}.${task.slot}` : serviceName;
}

/** The swarmnode graph id of the local daemon's node, if it is in the graph. */
function localGroupId(dgNodes: DGNode[], localNodeId: string | null | undefined): string | undefined {
  if (!localNodeId) return undefined;
  return dgNodes.find((n) => n.type === 'swarmnode' && n.swarmNode?.id === localNodeId)?.id;
}

/** Role of a swarm node in the per-node view; anything but a manager is a worker. */
export function swarmRole(n: DGNode): SwarmRole {
  return n.swarmNode?.role === 'manager' ? 'manager' : 'worker';
}

/** Id of the "Managers" / "Workers" container of the per-node view. */
export function roleGroupId(role: SwarmRole): string {
  return `rolegroup:${role}`;
}

/** Hostname segment used in card ids for tasks without a node. */
const UNASSIGNED_CARD_HOST = '*';

/**
 * Stable id of the card for one service on one swarm node. It depends only on
 * hostname and service name, so it survives task restarts and state changes.
 */
export function nodeServiceCardId(hostname: string, serviceName: string): string {
  return `nodesvc:${hostname}:${serviceName}`;
}

/** True when a swarm node isn't taking work: down/disconnected, drained or paused. */
export function isSwarmNodeInactive(n: DGNode): boolean {
  const info = n.swarmNode;
  const state = info?.state ?? n.status;
  if (state !== undefined && state !== 'ready') return true;
  return info?.availability === 'drain' || info?.availability === 'pause';
}

/** Splits a node's sorted placed tasks into consecutive runs per service. */
function groupByService(tasks: PlacedTask[]): { service: DGNode; tasks: TaskInfo[] }[] {
  const out: { service: DGNode; tasks: TaskInfo[] }[] = [];
  for (const { service, task } of tasks) {
    const last = out[out.length - 1];
    if (last && last.service.id === service.id) last.tasks.push(task);
    else out.push({ service, tasks: [task] });
  }
  return out;
}

/**
 * Converts the (stack-scoped) graph into the per-swarm-node view, nested
 * three levels deep with React Flow parentIds:
 *
 *   roleGroup ("Managers" / "Workers") → nodeGroup (one swarm node)
 *     → nodeServiceCard (one service on that node, holding its tasks)
 *
 * A role group is omitted when it has no nodes. Tasks the scheduler couldn't
 * place go into a free-standing Unassigned box. Standalone containers go into
 * the box of the local node (`localNodeId`, from /api/system/info), or stay
 * free-standing while it is unknown. Networks and volumes are hidden and no
 * edges are drawn: network wiring means nothing in a placement view.
 *
 * Positions are left at the origin; layout/nodeLayout.ts places everything.
 * Parents always precede their children, as React Flow requires.
 */
export function toNodeGroupedFlowNodes(dgNodes: DGNode[], localNodeId?: string | null): RFNode[] {
  const swarmNodes = listSwarmNodes(dgNodes);
  const placed = placeTasks(dgNodes);
  const localGroup = localGroupId(dgNodes, localNodeId);
  const containers = dgNodes.filter((n) => n.type === 'container' && n.status !== 'not_running');

  const roles: RFNode[] = [];
  const boxes: RFNode[] = [];
  const cards: RFNode[] = [];

  const addBox = (id: string, dgNode: DGNode, parentId: string | undefined, role: SwarmRole | undefined): number => {
    const tasks = placed.get(id) ?? [];
    const local = id === localGroup ? containers : [];
    const taskCount = tasks.length + local.length;
    const data: SwarmNodeGroupData = { dgNode, taskCount };
    if (role) data.role = role;
    else data.unassigned = true;
    boxes.push({
      id,
      type: 'nodeGroup',
      position: { x: 0, y: 0 },
      ...(parentId ? { parentId, extent: 'parent' as const } : null),
      draggable: false,
      data: data as unknown as Record<string, unknown>,
    });
    const host = role ? dgNode.name : UNASSIGNED_CARD_HOST;
    const inactive = role ? isSwarmNodeInactive(dgNode) : false;
    for (const { service, tasks: svcTasks } of groupByService(tasks)) {
      const cardId = nodeServiceCardId(host, service.name);
      const stack = projectOf(service);
      const cardData: NodeServiceCardData = {
        dgNode: { id: cardId, type: 'service', name: service.name, status: service.status, stack },
        serviceId: service.id,
        serviceName: service.name,
        stack,
        tasks: svcTasks,
      };
      if (inactive) cardData.nodeInactive = true;
      cards.push({
        id: cardId,
        type: 'nodeServiceCard',
        position: { x: 0, y: 0 },
        parentId: id,
        extent: 'parent',
        draggable: false,
        data: cardData as unknown as Record<string, unknown>,
      });
    }
    for (const c of local) {
      cards.push({
        id: c.id,
        type: 'containerNode',
        position: { x: 0, y: 0 },
        parentId: id,
        extent: 'parent',
        draggable: false,
        data: { dgNode: c },
      });
    }
    return taskCount;
  };

  let hasManagers = false;
  let workers: DGNode[] = [];
  for (const role of ['manager', 'worker'] as const) {
    const members = swarmNodes.filter((n) => swarmRole(n) === role);
    if (members.length === 0) continue;
    if (role === 'manager') hasManagers = true;
    else workers = members;
    const gid = roleGroupId(role);
    const group: RFNode = {
      id: gid,
      type: 'roleGroup',
      position: { x: 0, y: 0 },
      // Role groups are the only draggable nodes of the view; their boxes and
      // cards follow through parentId.
      draggable: true,
      data: {},
    };
    roles.push(group);
    let taskCount = 0;
    for (const n of members) taskCount += addBox(n.id, n, gid, role);
    const data: RoleGroupData = { role, nodeCount: members.length, taskCount };
    group.data = data as unknown as Record<string, unknown>;
  }

  // Control-plane summary badge, free-standing in the gap between the two
  // groups (see layout/nodeLayout.ts for its position). Only when both a
  // manager and at least one worker exist, matching the old aggregate link.
  if (hasManagers && workers.length > 0) {
    const ready = workers.filter((n) => (n.swarmNode?.state ?? n.status) === 'ready').length;
    const data: ControlSummaryData = { ready, total: workers.length, healthy: ready === workers.length };
    roles.push({
      id: CONTROL_SUMMARY_ID,
      type: 'controlSummary',
      position: { x: 0, y: 0 },
      draggable: false,
      selectable: false,
      focusable: false,
      data: data as unknown as Record<string, unknown>,
    });
  }
  if (placed.has(UNASSIGNED_NODE_GROUP_ID)) {
    addBox(
      UNASSIGNED_NODE_GROUP_ID,
      { id: UNASSIGNED_NODE_GROUP_ID, type: 'swarmnode', name: 'Unassigned' },
      undefined,
      undefined,
    );
  }

  // Without a known local node, standalone containers stay free-standing.
  const free: RFNode[] = localGroup
    ? []
    : containers.map((c) => ({
        id: c.id, type: 'containerNode', position: { x: 0, y: 0 }, draggable: false, data: { dgNode: c },
      }));

  return [...roles, ...boxes, ...cards, ...free];
}

/**
 * Topology fingerprint of the per-node view: which boxes and cards exist,
 * which parent holds each, and how many task rows each card has (its height).
 * Task state changes and restarts that keep the per-node count leave it
 * unchanged, so they don't relayout (and don't reset dragged groups).
 */
export function nodeGroupedTopologyKey(dgNodes: DGNode[], localNodeId?: string | null): string {
  return toNodeGroupedFlowNodes(dgNodes, localNodeId)
    .map((n) => {
      const rows = n.type === 'nodeServiceCard' ? (n.data as unknown as NodeServiceCardData).tasks.length : 0;
      return `${n.id}>${n.parentId ?? ''}#${rows}`;
    })
    .sort()
    .join(',');
}

/** Node usage against its capacity, as percentages (undefined when unknown). */
export interface NodeUsage {
  /** CPU cores the node offers (NanoCPUs / 1e9). */
  cores?: number;
  cpuPercent?: number;
  memPercent?: number;
}

/**
 * Relates a node's aggregate stats to its capacity. Docker CPU percentages
 * count 100% per core, so the share of the node is cpuPercent / cores.
 */
export function nodeUsage(
  info: { nanoCpus?: number; memoryBytes?: number } | undefined,
  stats: { cpuPercent: number; memUsage: number } | undefined,
): NodeUsage {
  const cores = info?.nanoCpus ? info.nanoCpus / 1e9 : undefined;
  if (!stats) return { cores };
  const clamp = (v: number) => Math.max(0, Math.min(100, v));
  return {
    cores,
    cpuPercent: cores ? clamp(stats.cpuPercent / cores) : undefined,
    memPercent: info?.memoryBytes ? clamp((stats.memUsage / info.memoryBytes) * 100) : undefined,
  };
}

/** Formats a core count: "4 cores", "0.5 cores", "1 core". */
export function formatCores(cores: number): string {
  const v = Number.isInteger(cores) ? String(cores) : cores.toFixed(1);
  return `${v} ${cores === 1 ? 'core' : 'cores'}`;
}
