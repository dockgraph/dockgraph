import type { Node as RFNode, Edge as RFEdge } from '@xyflow/react';
import {
  FADE_OPACITY,
  EDGE_FADE_OPACITY,
  HIGHLIGHT_EDGE_STROKE_WIDTH,
  DEFAULT_EDGE_STROKE_WIDTH,
} from '../utils/constants';
import { isGroupType } from '../layout/elkGraph';

export interface SelectionState {
  type: 'node' | 'edge';
  id: string;
}

interface ConnectedElements {
  connectedEdgeIds: Set<string>;
  connectedNodeIds: Set<string>;
  highlightedGroupIds: Set<string>;
}

/** Service graph id carried by a per-node service card, if `n` is one. */
export function serviceIdOf(n: RFNode | null | undefined): string | undefined {
  if (n?.type !== 'nodeServiceCard') return undefined;
  return (n.data as { serviceId?: string }).serviceId;
}

/** Ids of every node nested (at any depth) under `groupId`. */
function descendantIds(groupId: string, nodes: RFNode[]): Set<string> {
  const byParent = new Map<string, string[]>();
  for (const n of nodes) {
    if (!n.parentId) continue;
    const list = byParent.get(n.parentId) ?? [];
    list.push(n.id);
    byParent.set(n.parentId, list);
  }
  const out = new Set<string>();
  const stack = [groupId];
  while (stack.length > 0) {
    for (const id of byParent.get(stack.pop()!) ?? []) {
      if (out.has(id)) continue;
      out.add(id);
      stack.push(id);
    }
  }
  return out;
}

/**
 * True when the selected node or edge exists in the current graph. After a
 * stack or grouping change the selection can name an element that is gone;
 * styling for it would fade everything else, so callers treat it as no
 * selection.
 */
export function isSelectionPresent(selection: SelectionState, nodes: RFNode[], edges: RFEdge[]): boolean {
  const list: { id: string }[] = selection.type === 'node' ? nodes : edges;
  return list.some((el) => el.id === selection.id);
}

/**
 * Walks the graph from the selected element to find all directly connected
 * nodes, edges, and parent groups that should remain fully visible.
 */
export function resolveConnectedElements(
  selection: SelectionState,
  nodes: RFNode[],
  edges: RFEdge[],
): ConnectedElements {
  const connectedEdgeIds = new Set<string>();
  const connectedNodeIds = new Set<string>();
  const highlightedGroupIds = new Set<string>();

  const selectedNode = selection.type === 'node'
    ? nodes.find((n) => n.id === selection.id)
    : null;
  const isGroupSelection = isGroupType(selectedNode?.type);

  const nodeById = new Map(nodes.map((n) => [n.id, n]));

  if (selection.type === 'node') {
    if (isGroupSelection) {
      // Group selection: highlight all children + edges touching children +
      // parent groups of remote endpoints for cross-group context.
      // Groups nest in the per-node view (role group → node box → cards),
      // so every descendant lights up, and nested groups stay visible.
      highlightedGroupIds.add(selection.id);
      const childIds = descendantIds(selection.id, nodes);
      for (const id of childIds) {
        connectedNodeIds.add(id);
        if (isGroupType(nodeById.get(id)?.type)) highlightedGroupIds.add(id);
      }
      // The selected group itself counts as inside: control links leave the
      // Managers role group directly, not one of its children.
      const inside = (id: string) => id === selection.id || childIds.has(id);
      for (const e of edges) {
        if (inside(e.source) || inside(e.target)) {
          connectedEdgeIds.add(e.id);
          connectedNodeIds.add(e.source);
          connectedNodeIds.add(e.target);
          // Light every group above the remote endpoint (node box → role group).
          const remoteId = inside(e.source) ? e.target : e.source;
          let parentId = nodeById.get(remoteId)?.parentId;
          while (parentId && !highlightedGroupIds.has(parentId)) {
            highlightedGroupIds.add(parentId);
            parentId = nodeById.get(parentId)?.parentId;
          }
        }
      }
    } else {
      // Single node: highlight directly connected edges and opposite endpoints.
      connectedNodeIds.add(selection.id);
      // A per-node service card also lights the service's cards on other
      // nodes, and the edges (swarm links) leaving any of them.
      const origins = new Set([selection.id]);
      const serviceId = serviceIdOf(selectedNode);
      if (serviceId) {
        for (const n of nodes) {
          if (serviceIdOf(n) === serviceId) {
            connectedNodeIds.add(n.id);
            origins.add(n.id);
          }
        }
      }
      for (const e of edges) {
        if (origins.has(e.source) || origins.has(e.target)) {
          connectedEdgeIds.add(e.id);
          connectedNodeIds.add(e.source);
          connectedNodeIds.add(e.target);
        }
      }
    }
  } else if (selection.type === 'edge') {
    const edge = edges.find((e) => e.id === selection.id);
    if (edge) {
      connectedEdgeIds.add(edge.id);
      connectedNodeIds.add(edge.source);
      connectedNodeIds.add(edge.target);

      // When an edge endpoint is a network group, include all its children
      // so clicking a node↔network edge lights up the entire network.
      for (const endpointId of [edge.source, edge.target]) {
        if (isGroupType(nodeById.get(endpointId)?.type)) {
          for (const n of nodes) {
            if (n.parentId === endpointId) connectedNodeIds.add(n.id);
          }
        }
      }
    }
  }

  // For non-group selections, mark parent groups of highlighted nodes as visible
  // so children don't appear highlighted inside a faded-out group.
  // Walks up nested groups too (card → node box → role group).
  if (!isGroupSelection) {
    for (const id of connectedNodeIds) {
      let parentId = nodeById.get(id)?.parentId;
      while (parentId && !highlightedGroupIds.has(parentId)) {
        highlightedGroupIds.add(parentId);
        parentId = nodeById.get(parentId)?.parentId;
      }
    }
  }

  return { connectedEdgeIds, connectedNodeIds, highlightedGroupIds };
}

/** Applies selection-based opacity and stroke width to nodes. */
export function styleNodesForSelection(
  nodes: RFNode[],
  connectedNodeIds: Set<string>,
  highlightedGroupIds: Set<string>,
): RFNode[] {
  return nodes.map((n) => {
    const highlighted = isGroupType(n.type)
      ? highlightedGroupIds.has(n.id) || connectedNodeIds.has(n.id)
      : connectedNodeIds.has(n.id);
    return { ...n, style: { ...n.style, opacity: highlighted ? 1 : FADE_OPACITY } };
  });
}

/** Applies selection-based opacity and stroke width to edges. */
export function styleEdgesForSelection(
  edges: RFEdge[],
  connectedEdgeIds: Set<string>,
  isLowZoom: boolean,
): RFEdge[] {
  return edges.map((e) => {
    const highlighted = connectedEdgeIds.has(e.id);
    return {
      ...e,
      style: {
        ...e.style,
        opacity: highlighted ? 1 : EDGE_FADE_OPACITY,
        strokeWidth: highlighted && isLowZoom ? HIGHLIGHT_EDGE_STROKE_WIDTH : DEFAULT_EDGE_STROKE_WIDTH,
      },
    };
  });
}
