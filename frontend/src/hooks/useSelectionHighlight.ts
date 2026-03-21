import { useCallback, useMemo, useState } from 'react';
import type { Node as RFNode, Edge as RFEdge } from '@xyflow/react';
import { FADE_OPACITY, EDGE_FADE_OPACITY } from '../utils/constants';

interface SelectionState {
  type: 'node' | 'edge';
  id: string;
}

interface HighlightResult {
  styledNodes: RFNode[];
  styledEdges: RFEdge[];
  onNodeClick: (_: React.MouseEvent, node: RFNode) => void;
  onEdgeClick: (_: React.MouseEvent, edge: RFEdge) => void;
  onPaneClick: () => void;
}

/**
 * Manages click-to-highlight behavior for graph elements.
 * When a node or edge is selected, connected elements stay fully opaque
 * while unrelated elements fade to 20% opacity.
 */
export function useSelectionHighlight(nodes: RFNode[], edges: RFEdge[]): HighlightResult {
  const [selection, setSelection] = useState<SelectionState | null>(null);

  const { styledNodes, styledEdges } = useMemo(() => {
    if (!selection) return { styledNodes: nodes, styledEdges: edges };

    // Three sets track what stays fully visible when something is selected:
    // edges directly involved, individual nodes, and their parent network groups.
    const connectedEdgeIds = new Set<string>();
    const connectedNodeIds = new Set<string>();
    const highlightedGroupIds = new Set<string>();
    const selectedNode = selection.type === 'node'
      ? nodes.find((n) => n.id === selection.id)
      : null;
    const isGroupSelection = selectedNode?.type === 'networkGroup';

    const nodeById = new Map(nodes.map((n) => [n.id, n]));

    if (selection.type === 'node') {
      // Group selection: highlight all children, any edges touching those children,
      // and the parent groups of remote endpoints so cross-group context is preserved.
      if (isGroupSelection) {
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
        // Single node: highlight its directly connected edges and their opposite endpoints.
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
      }
    }

    // For non-group selections, mark the parent group of every highlighted node
    // as visible so children don't appear highlighted inside a faded-out group.
    if (!isGroupSelection) {
      for (const n of nodes) {
        if (connectedNodeIds.has(n.id) && n.parentId) {
          highlightedGroupIds.add(n.parentId);
        }
      }
    }

    return {
      styledNodes: nodes.map((n) => {
        const highlighted = n.type === 'networkGroup'
          ? highlightedGroupIds.has(n.id)
          : connectedNodeIds.has(n.id);
        return { ...n, style: { ...n.style, opacity: highlighted ? 1 : FADE_OPACITY } };
      }),
      styledEdges: edges.map((e) => ({
        ...e,
        style: { ...e.style, opacity: connectedEdgeIds.has(e.id) ? 1 : EDGE_FADE_OPACITY },
      })),
    };
  }, [selection, nodes, edges]);

  const onNodeClick = useCallback((_: React.MouseEvent, node: RFNode) => {
    setSelection((prev) =>
      prev?.type === 'node' && prev.id === node.id ? null : { type: 'node', id: node.id },
    );
  }, []);

  const onEdgeClick = useCallback((_: React.MouseEvent, edge: RFEdge) => {
    setSelection((prev) =>
      prev?.type === 'edge' && prev.id === edge.id ? null : { type: 'edge', id: edge.id },
    );
  }, []);

  const onPaneClick = useCallback(() => {
    setSelection(null);
  }, []);

  return { styledNodes, styledEdges, onNodeClick, onEdgeClick, onPaneClick };
}
