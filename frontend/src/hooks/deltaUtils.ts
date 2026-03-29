import type { DGNode, DGEdge, DeltaUpdate } from '../types';

/**
 * Produces a lightweight fingerprint of the current graph state.
 * Prevents redundant re-renders when the WebSocket pushes a snapshot
 * that is structurally identical to what we already have (same node IDs,
 * same statuses, same edge set). IDs are sorted so order differences
 * between snapshots don't cause false mismatches.
 */
export function snapshotFingerprint(nodes: DGNode[], edges: DGEdge[]): string {
  const nk = (nodes ?? [])
    .map(
      (n) =>
        `${n.id}:${n.status ?? ''}:${n.image ?? ''}:${n.networkId ?? ''}:${(n.ports ?? []).map((p) => `${p.host}-${p.container}`).join(';')}`,
    )
    .sort()
    .join(',');
  const ek = (edges ?? [])
    .map((e) => e.id)
    .sort()
    .join(',');
  return nk + '|' + ek;
}

/**
 * Applies a delta update to the previous graph state, returning a new
 * state object. Pure function — does not mutate the input.
 */
export function applyDelta(
  prev: { nodes: DGNode[]; edges: DGEdge[] },
  delta: DeltaUpdate,
): { nodes: DGNode[]; edges: DGEdge[] } {
  let nodes = [...prev.nodes];
  let edges = [...prev.edges];

  if (delta.nodesRemoved) {
    const removed = new Set(delta.nodesRemoved);
    nodes = nodes.filter((n) => !removed.has(n.id));
  }

  if (delta.nodesAdded) {
    nodes = [...nodes, ...delta.nodesAdded];
  }

  if (delta.nodesUpdated) {
    for (const update of delta.nodesUpdated) {
      const idx = nodes.findIndex((n) => n.id === update.id);
      if (idx !== -1) {
        nodes[idx] = { ...nodes[idx], ...update } as DGNode;
      }
    }
  }

  if (delta.edgesRemoved) {
    const removed = new Set(delta.edgesRemoved);
    edges = edges.filter((e) => !removed.has(e.id));
  }

  if (delta.edgesAdded) {
    edges = [...edges, ...delta.edgesAdded];
  }

  return { nodes, edges };
}
