import { useEffect, useMemo, useRef, useState } from 'react';
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
): GraphLayoutResult {
  const [nodes, setNodes, onNodesChange] = useNodesState<RFNode>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<RFEdge>([]);

  const topoKey = useMemo(() => topologyKey(dgNodes, dgEdges), [dgNodes, dgEdges]);
  const prevTopoKeyRef = useRef('');
  const layoutInFlightRef = useRef(false);
  const [layoutBusy, setLayoutBusy] = useState(false);
  const [layoutError, setLayoutError] = useState(false);

  // Full ELK layout — only when topology (node/edge set) changes.
  useEffect(() => {
    if (dgNodes.length === 0) return;
    let cancelled = false;
    layoutInFlightRef.current = true;

    setLayoutBusy(true);
    setLayoutError(false);
    const rfNodes = toReactFlowNodes(dgNodes, dgEdges);
    const rfEdges = toReactFlowEdges(dgEdges, dgNodes, edgeStroke);

    computeLayout(rfNodes, rfEdges)
      .then((layout) => {
        if (cancelled) return;
        prevTopoKeyRef.current = topoKey;
        setNodes(layout.nodes);
        setEdges(layout.edges);
      })
      .catch((err) => {
        console.error('layout computation failed:', err);
        if (!cancelled) setLayoutError(true);
      })
      .finally(() => {
        if (!cancelled) {
          setLayoutBusy(false);
          layoutInFlightRef.current = false;
        }
      });

    return () => {
      cancelled = true;
      layoutInFlightRef.current = false;
    };
  // topoKey captures the identity of dgNodes/dgEdges — when it changes, the
  // full layout runs. edgeStroke is excluded because color-only changes are
  // handled by the lightweight update below.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [topoKey, setNodes, setEdges]);

  // Lightweight update — apply status/style changes without relayout.
  useEffect(() => {
    if (dgNodes.length === 0 || topoKey !== prevTopoKeyRef.current) return;
    if (layoutInFlightRef.current) return;

    const rfEdges = toReactFlowEdges(dgEdges, dgNodes, edgeStroke);
    const rfEdgeMap = new Map(rfEdges.map((e) => [e.id, e]));
    setEdges((prev) => prev.map((e) => {
      const updated = rfEdgeMap.get(e.id);
      return updated ? { ...e, data: { ...e.data, ...updated.data }, style: updated.style } : e;
    }));

    const rfNodes = toReactFlowNodes(dgNodes, dgEdges);
    const rfNodeMap = new Map(rfNodes.map((n) => [n.id, n]));
    setNodes((prev) => prev.map((n) => {
      const updated = rfNodeMap.get(n.id);
      return updated ? { ...n, data: updated.data } : n;
    }));
  // dgNodes/dgEdges trigger this on every data change. When the topology
  // also changed, the guard (topoKey !== prevTopoKeyRef) skips this effect
  // in favour of the full layout above.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [dgNodes, dgEdges, edgeStroke, setNodes, setEdges]);

  // Cover the render gap between "input nodes arrived" and "effect sets
  // layoutBusy=true": if we have input but no positioned output yet and
  // no error, the layout is effectively pending.
  const effectiveBusy = layoutBusy || (dgNodes.length > 0 && nodes.length === 0 && !layoutError);

  return { nodes, edges, setNodes, setEdges, onNodesChange, onEdgesChange, layoutBusy: effectiveBusy, layoutError };
}
