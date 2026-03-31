import type { Node as RFNode, Edge as RFEdge } from '@xyflow/react';
import {
  FADE_OPACITY,
  EDGE_FADE_OPACITY,
  HIGHLIGHT_EDGE_STROKE_WIDTH,
  DEFAULT_EDGE_STROKE_WIDTH,
} from '../utils/constants';

export interface SelectionState {
  type: 'node' | 'edge';
  id: string;
}

interface ConnectedElements {
  connectedEdgeIds: Set<string>;
  connectedNodeIds: Set<string>;
  highlightedGroupIds: Set<string>;
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
  const isGroupSelection = selectedNode?.type === 'networkGroup';

  const nodeById = new Map(nodes.map((n) => [n.id, n]));

  if (selection.type === 'node') {
    if (isGroupSelection) {
      // Group selection: highlight all children + edges touching children +
      // parent groups of remote endpoints for cross-group context.
      highlightedGroupIds.add(selection.id);
      const childIds = new Set(
        nodes.filter((n) => n.parentId === selection.id).map((n) => n.id),
      );
      for (const id of childIds) connectedNodeIds.add(id);
      for (const e of edges) {
        if (childIds.has(e.source) || childIds.has(e.target) ||
            e.source === selection.id || e.target === selection.id) {
          connectedEdgeIds.add(e.id);
          connectedNodeIds.add(e.source);
          connectedNodeIds.add(e.target);
          const remoteId = childIds.has(e.source) ? e.target : e.source;
          const remoteNode = nodeById.get(remoteId);
          if (remoteNode?.parentId) highlightedGroupIds.add(remoteNode.parentId);
        }
      }
    } else {
      // Single node: highlight directly connected edges and opposite endpoints.
      connectedNodeIds.add(selection.id);
      for (const e of edges) {
        if (e.source === selection.id || e.target === selection.id) {
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
        if (nodeById.get(endpointId)?.type === 'networkGroup') {
          for (const n of nodes) {
            if (n.parentId === endpointId) connectedNodeIds.add(n.id);
          }
        }
      }
    }
  }

  // For non-group selections, mark parent groups of highlighted nodes as visible
  // so children don't appear highlighted inside a faded-out group.
  if (!isGroupSelection) {
    for (const n of nodes) {
      if (connectedNodeIds.has(n.id) && n.parentId) {
        highlightedGroupIds.add(n.parentId);
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
    const highlighted = n.type === 'networkGroup'
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
