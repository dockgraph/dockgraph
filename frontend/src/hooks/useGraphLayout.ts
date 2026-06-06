import { useEffect, useMemo, useState } from 'react';
import {
  useNodesState,
  useEdgesState,
  type Node as RFNode,
  type Edge as RFEdge,
} from '@xyflow/react';
import { computeLayout } from '../layout/elk';
import { toReactFlowNodes, toReactFlowEdges } from '../utils/graphTransform';
import type { DGNode, DGEdge } from '../types';

interface GraphLayoutResult {
  nodes: RFNode[];
  edges: RFEdge[];
  setNodes: ReturnType<typeof useNodesState<RFNode>>[1];
  setEdges: ReturnType<typeof useEdgesState<RFEdge>>[1];
  onNodesChange: ReturnType<typeof useNodesState<RFNode>>[2];
  onEdgesChange: ReturnType<typeof useEdgesState<RFEdge>>[2];
  layoutBusy: boolean;
  layoutError: boolean;
}

/**
 * Topology fingerprint — changes only when nodes or edges are added/removed.
 * Status changes (running -> exited) don't alter the fingerprint, so they
 * skip the expensive ELK layout and only update node/edge data in place.
 */
function topologyKey(dgNodes: DGNode[], dgEdges: DGEdge[]): string {
  const nk = dgNodes.map((n) => n.id).sort().join(',');
  const ek = dgEdges.map((e) => e.id).sort().join(',');
  return nk + '|' + ek;
}

/**
 * Manages ELK layout computation and lightweight in-place updates.
 *
 * When the topology changes (nodes/edges added or removed), runs a full
 * async ELK layout. When only data changes (status, ports, etc.), patches
 * the existing positioned nodes/edges without relayout.
 */
export function useGraphLayout(
  dgNodes: DGNode[],
  dgEdges: DGEdge[],
  edgeStroke: string,
  accentStroke: string,
): GraphLayoutResult {
  const [nodes, setNodes, onNodesChange] = useNodesState<RFNode>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<RFEdge>([]);

  const topoKey = useMemo(() => topologyKey(dgNodes, dgEdges), [dgNodes, dgEdges]);
  // settledTopoKey: fingerprint of the topology currently positioned on screen.
  // errorTopoKey:   fingerprint of the topology whose most recent attempt failed.
  // Tracking these as state lets layoutBusy/layoutError be derived during render
  // instead of being assigned via setState inside the effect body.
  const [settledTopoKey, setSettledTopoKey] = useState('');
  const [errorTopoKey, setErrorTopoKey] = useState<string | null>(null);
  const layoutBusy = dgNodes.length > 0 && topoKey !== settledTopoKey && errorTopoKey !== topoKey;
  const layoutError = errorTopoKey === topoKey;

  // Full ELK layout — only when topology (node/edge set) changes.
  useEffect(() => {
    if (dgNodes.length === 0) return;
    let cancelled = false;

    const rfNodes = toReactFlowNodes(dgNodes, dgEdges);
    const rfEdges = toReactFlowEdges(dgEdges, dgNodes, edgeStroke, accentStroke);

    computeLayout(rfNodes, rfEdges)
      .then((layout) => {
        if (cancelled) return;
        setNodes(layout.nodes);
        setEdges(layout.edges);
        setErrorTopoKey(null);
        setSettledTopoKey(topoKey);
      })
      .catch((err) => {
        console.error('layout computation failed:', err);
        if (!cancelled) setErrorTopoKey(topoKey);
      });

    return () => { cancelled = true; };
  // edgeStroke is excluded — color-only changes are handled by the
  // lightweight update below without re-running ELK.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [topoKey, setNodes, setEdges]);

  // Lightweight update — apply status/style changes without relayout.
  // settledTopoKey is in deps so this re-runs after a fresh layout completes,
  // catching data that arrived while the layout was in flight.
  useEffect(() => {
    if (dgNodes.length === 0 || topoKey !== settledTopoKey) return;

    const rfEdges = toReactFlowEdges(dgEdges, dgNodes, edgeStroke, accentStroke);
    const rfEdgeMap = new Map(rfEdges.map((e) => [e.id, e]));
    setEdges((prev) => prev.map((e) => {
      const updated = rfEdgeMap.get(e.id);
      return updated ? { ...e, data: { ...e.data, ...updated.data }, style: updated.style } : e;
    }));

    const rfNodes = toReactFlowNodes(dgNodes, dgEdges);
    const rfNodeMap = new Map(rfNodes.map((n) => [n.id, n]));
    setNodes((prev) => prev.map((n) => {
      const updated = rfNodeMap.get(n.id);
      return updated ? { ...n, data: { ...n.data, ...updated.data } } : n;
    }));
  }, [dgNodes, dgEdges, edgeStroke, accentStroke, topoKey, settledTopoKey, setNodes, setEdges]);

  return { nodes, edges, setNodes, setEdges, onNodesChange, onEdgesChange, layoutBusy, layoutError };
}
