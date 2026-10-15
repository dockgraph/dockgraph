import { useCallback, useEffect, useMemo, useState } from 'react';
import type { Node as RFNode, Edge as RFEdge } from '@xyflow/react';
import { useStore } from '@xyflow/react';
import { FADE_OPACITY, zoomSelector } from '../utils/constants';
import {
  type SelectionState,
  isSelectionPresent,
  resolveConnectedElements,
  styleNodesForSelection,
  styleEdgesForSelection,
  serviceIdOf,
} from '../utils/selectionGraph';
import { isGroupType } from '../layout/elkGraph';

interface HighlightResult {
  styledNodes: RFNode[];
  styledEdges: RFEdge[];
  canvasEdges: RFEdge[];
  svgEdges: RFEdge[];
  /** Selection-styled edges from `linkEdgesFor`; always drawn as SVG. */
  linkEdges: RFEdge[];
  onNodeClick: (_: React.MouseEvent, node: RFNode) => void;
  onEdgeClick: (_: React.MouseEvent, edge: RFEdge) => void;
  onPaneClick: () => void;
  selectNode: (id: string) => void;
  selectEdge: (id: string) => void;
}

/**
 * True when a node stays lit under an active search: groups always do,
 * per-node service cards follow their service.
 */
export function matchesSearch(n: RFNode, matchingNodeIds: Set<string>): boolean {
  if (isGroupType(n.type) || matchingNodeIds.has(n.id)) return true;
  const serviceId = serviceIdOf(n);
  return serviceId !== undefined && matchingNodeIds.has(serviceId);
}

/**
 * Manages click-to-highlight behavior for graph elements.
 * When a node or edge is selected, connected elements stay fully opaque
 * while unrelated elements fade to 20% opacity.
 */
/**
 * `linkEdgesFor` adds edges that depend on the selection (the per-node view's
 * swarm links): they take part in highlighting, so the nodes they reach stay
 * lit, and come back styled in `linkEdges`.
 */
export function useSelectionHighlight(
  nodes: RFNode[],
  edges: RFEdge[],
  useCanvas = false,
  matchingNodeIds: Set<string> | null = null,
  linkEdgesFor?: (selection: SelectionState | null) => RFEdge[],
): HighlightResult {
  const [selection, setSelection] = useState<SelectionState | null>(null);
  const isLowZoom = useStore(zoomSelector);

  const { styledNodes, styledEdges, canvasEdges, svgEdges, linkEdges } = useMemo(() => {
    let active = selection;
    let links = linkEdgesFor?.(active) ?? [];
    // A selection left over from a previous topology (stack or grouping
    // change) is ignored rather than fading the whole new graph. It is kept,
    // not cleared, so a node that comes back after an async relayout stays
    // selected. Selection-dependent links count, so a selected link edge
    // stays active.
    if (active && !isSelectionPresent(active, nodes, links.length > 0 ? [...edges, ...links] : edges)) {
      active = null;
      links = linkEdgesFor?.(null) ?? [];
    }
    if (!active) {
      // When search is active but no selection, dim non-matching nodes.
      if (matchingNodeIds) {
        const searchStyled = nodes.map((n) => ({
          ...n,
          style: { ...n.style, opacity: matchesSearch(n, matchingNodeIds) ? 1 : FADE_OPACITY },
        }));
        return {
          styledNodes: searchStyled,
          styledEdges: edges,
          canvasEdges: useCanvas ? edges : [],
          svgEdges: useCanvas ? [] as RFEdge[] : [],
          linkEdges: links,
        };
      }
      return {
        styledNodes: nodes,
        styledEdges: edges,
        canvasEdges: useCanvas ? edges : [],
        svgEdges: useCanvas ? [] as RFEdge[] : [],
        linkEdges: links,
      };
    }

    const { connectedEdgeIds, connectedNodeIds, highlightedGroupIds } =
      resolveConnectedElements(active, nodes, links.length > 0 ? [...edges, ...links] : edges);

    const styledEdgeList = styleEdgesForSelection(edges, connectedEdgeIds, isLowZoom);

    return {
      styledNodes: styleNodesForSelection(nodes, connectedNodeIds, highlightedGroupIds),
      styledEdges: styledEdgeList,
      // Canvas mode: all edges stay on canvas with selection opacity applied.
      canvasEdges: useCanvas ? styledEdgeList : [],
      svgEdges: [],
      linkEdges: styleEdgesForSelection(links, connectedEdgeIds, isLowZoom),
    };
  }, [selection, nodes, edges, useCanvas, isLowZoom, matchingNodeIds, linkEdgesFor]);

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

  const selectNode = useCallback((id: string) => {
    setSelection({ type: 'node', id });
  }, []);

  const selectEdge = useCallback((id: string) => {
    setSelection((prev) =>
      prev?.type === 'edge' && prev.id === id ? null : { type: 'edge', id },
    );
  }, []);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setSelection(null);
    };
    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, []);

  return { styledNodes, styledEdges, canvasEdges, svgEdges, linkEdges, onNodeClick, onEdgeClick, onPaneClick, selectNode, selectEdge };
}
