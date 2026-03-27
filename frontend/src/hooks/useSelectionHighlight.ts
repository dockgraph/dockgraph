import { useCallback, useEffect, useMemo, useState } from 'react';
import type { Node as RFNode, Edge as RFEdge } from '@xyflow/react';
import { useStore } from '@xyflow/react';
import { LOD_ZOOM_THRESHOLD } from '../utils/constants';
import {
  type SelectionState,
  resolveConnectedElements,
  styleNodesForSelection,
  styleEdgesForSelection,
} from './selectionGraph';

interface HighlightResult {
  styledNodes: RFNode[];
  styledEdges: RFEdge[];
  canvasEdges: RFEdge[];
  svgEdges: RFEdge[];
  onNodeClick: (_: React.MouseEvent, node: RFNode) => void;
  onEdgeClick: (_: React.MouseEvent, edge: RFEdge) => void;
  onPaneClick: () => void;
}

const zoomSelector = (s: { transform: [number, number, number] }) => s.transform[2] < LOD_ZOOM_THRESHOLD;

/**
 * Manages click-to-highlight behavior for graph elements.
 * When a node or edge is selected, connected elements stay fully opaque
 * while unrelated elements fade to 20% opacity.
 */
export function useSelectionHighlight(nodes: RFNode[], edges: RFEdge[], useCanvas = false): HighlightResult {
  const [selection, setSelection] = useState<SelectionState | null>(null);
  const isLowZoom = useStore(zoomSelector);

  const { styledNodes, styledEdges, canvasEdges, svgEdges } = useMemo(() => {
    if (!selection) {
      return {
        styledNodes: nodes,
        styledEdges: edges,
        canvasEdges: useCanvas ? edges : [],
        svgEdges: useCanvas ? [] as RFEdge[] : [],
      };
    }

    const { connectedEdgeIds, connectedNodeIds, highlightedGroupIds } =
      resolveConnectedElements(selection, nodes, edges);

    const styledEdgeList = styleEdgesForSelection(edges, connectedEdgeIds, isLowZoom);

    return {
      styledNodes: styleNodesForSelection(nodes, connectedNodeIds, highlightedGroupIds),
      styledEdges: styledEdgeList,
      // Canvas mode: all edges stay on canvas with selection opacity applied.
      canvasEdges: useCanvas ? styledEdgeList : [],
      svgEdges: [],
    };
  }, [selection, nodes, edges, useCanvas, isLowZoom]);

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

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setSelection(null);
    };
    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, []);

  return { styledNodes, styledEdges, canvasEdges, svgEdges, onNodeClick, onEdgeClick, onPaneClick };
}
