import type { Node as RFNode, Edge as RFEdge } from '@xyflow/react';
import { listSwarmNodes, roleGroupId, swarmRole } from './nodeTransform';
import { serviceIdOf } from './selectionGraph';
import type { DGEdge, DGNode } from '../types';

/** React Flow edge type of the per-node view's links. */
export const SWARM_LINK_EDGE_TYPE = 'swarmLink';

/** Id of one manager→worker control-plane spoke. */
export function controlLinkId(workerId: string): string {
  return `swarmlink:control:${workerId}`;
}

/** Most overlay links drawn for one selection, to keep the view readable. */
export const MAX_OVERLAY_LINKS = 60;

/** Swarm manager port the workers keep their control-plane session on. */
export const SWARM_CONTROL_PORT = 2377;

export interface ControlLinkData {
  kind: 'control';
  /** This worker's own connection state — "ready" (drained nodes still count). */
  healthy: boolean;
}

export interface OverlayLinkData {
  kind: 'overlay';
  /** Names of the networks both ends are attached to. */
  networks: string[];
  /** Stack of the selected service, for network colours. */
  stack?: string;
}

export type SwarmLinkData = ControlLinkData | OverlayLinkData;

/**
 * The control-plane links of the per-node view: one spoke per worker, from
 * the Managers group to that worker's box. The Docker API doesn't say which
 * manager a worker is attached to, so every spoke starts at the group rather
 * than a specific manager box — but fanning out to each worker, instead of
 * joining the two groups with a single aggregate line, shows which workers
 * are actually connected rather than just a ready/total count. Empty unless
 * both a manager and at least one worker exist.
 */
export function controlLinkEdges(dgNodes: DGNode[]): RFEdge[] {
  const nodes = listSwarmNodes(dgNodes);
  const workers = nodes.filter((n) => swarmRole(n) === 'worker');
  if (workers.length === 0 || !nodes.some((n) => swarmRole(n) === 'manager')) return [];
  const source = roleGroupId('manager');
  return workers.map((w) => {
    const healthy = (w.swarmNode?.state ?? w.status) === 'ready';
    const data: ControlLinkData = { kind: 'control', healthy };
    return {
      id: controlLinkId(w.id),
      type: SWARM_LINK_EDGE_TYPE,
      source,
      target: w.id,
      selectable: false,
      focusable: false,
      data: data as unknown as Record<string, unknown>,
    };
  });
}

/** Network names each service is attached to: its primary network plus secondary ones. */
function serviceNetworks(dgNodes: DGNode[], dgEdges: DGEdge[]): Map<string, Set<string>> {
  const nameOf = new Map(dgNodes.filter((n) => n.type === 'network').map((n) => [n.id, n.name]));
  const out = new Map<string, Set<string>>();
  const add = (serviceId: string, networkId: string | undefined) => {
    const name = networkId && nameOf.get(networkId);
    if (!name) return;
    const set = out.get(serviceId) ?? new Set<string>();
    set.add(name);
    out.set(serviceId, set);
  };
  for (const n of dgNodes) if (n.type === 'service') add(n.id, n.networkId);
  for (const e of dgEdges) if (e.type === 'secondary_network') add(e.source, e.target);
  return out;
}

/** Hostname segment of a `nodesvc:{host}:{service}` card id. */
function cardHost(cardId: string): string {
  return cardId.split(':')[1] ?? '';
}

/**
 * Overlay links for a selected service in the per-node view: one edge from
 * each card of the service to each card of a service sharing a network with
 * it (and between its own replicas) on another node. Same-node pairs are
 * skipped — that traffic never leaves the host. Capped at MAX_OVERLAY_LINKS.
 */
export function overlayLinkEdges(
  dgNodes: DGNode[],
  dgEdges: DGEdge[],
  rfNodes: RFNode[],
  serviceId: string,
): RFEdge[] {
  const networks = serviceNetworks(dgNodes, dgEdges);
  const own = networks.get(serviceId);
  if (!own || own.size === 0) return [];
  const stack = dgNodes.find((n) => n.id === serviceId)?.stack;

  const cards = rfNodes.filter((n) => serviceIdOf(n) !== undefined);
  const selected = cards.filter((n) => serviceIdOf(n) === serviceId);
  const peers: { card: RFNode; shared: string[] }[] = [];
  for (const card of cards) {
    const sid = serviceIdOf(card)!;
    const shared = sid === serviceId ? [...own] : [...(networks.get(sid) ?? [])].filter((net) => own.has(net));
    if (shared.length > 0) peers.push({ card, shared: shared.sort() });
  }

  const edges: RFEdge[] = [];
  const seen = new Set<string>();
  for (const from of selected) {
    for (const { card: to, shared } of peers) {
      if (to.id === from.id || cardHost(to.id) === cardHost(from.id)) continue;
      const key = [from.id, to.id].sort().join('|');
      if (seen.has(key)) continue;
      seen.add(key);
      const data: OverlayLinkData = { kind: 'overlay', networks: shared, stack };
      edges.push({
        id: `swarmlink:overlay:${key}`,
        type: SWARM_LINK_EDGE_TYPE,
        source: from.id,
        target: to.id,
        selectable: false,
        focusable: false,
        data: data as unknown as Record<string, unknown>,
      });
      if (edges.length >= MAX_OVERLAY_LINKS) return edges;
    }
  }
  return edges;
}
