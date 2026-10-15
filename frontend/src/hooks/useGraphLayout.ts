import { useEffect, useMemo, useState } from 'react';
import {
  useNodesState,
  useEdgesState,
  type Node as RFNode,
  type Edge as RFEdge,
} from '@xyflow/react';
import { computeLayout } from '../layout/elk';
import { layoutNodeGroups } from '../layout/nodeLayout';
import { toReactFlowNodes, toReactFlowEdges } from '../utils/graphTransform';
import { toNodeGroupedFlowNodes, nodeGroupedTopologyKey } from '../utils/nodeTransform';
import type { GroupBy } from './useGroupBy';
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
 * Topology fingerprint — changes when nodes or edges are added/removed, or a
 * workload moves to another network group.
 * Status changes (running -> exited) don't alter the fingerprint, so they
 * skip the expensive ELK layout and only update node/edge data in place.
 */
function topologyKey(dgNodes: DGNode[], dgEdges: DGEdge[], groupBy: GroupBy, localNodeId: string | null): string {
  if (groupBy === 'node') {
    // Tasks live inside their service node, so the per-node view keys on the
    // cards it will draw and the box holding each.
    return 'node|' + nodeGroupedTopologyKey(dgNodes, localNodeId);
  }
  // A workload's network group (its networkId, or the unmanaged group when it
  // has neither a network nor a compose source) becomes its React Flow
  // parentId, which the lightweight update doesn't touch, so it is part of the
  // topology.
  const nk = dgNodes
    .map((n) => `${n.id}>${n.networkId ?? (n.source ? '' : '!')}`)
    .sort()
    .join(',');
  const ek = dgEdges.map((e) => e.id).sort().join(',');
  return 'network|' + nk + '|' + ek;
}

/** React Flow nodes and edges for the chosen grouping. Node mode draws no edges. */
function buildFlow(
  dgNodes: DGNode[],
  dgEdges: DGEdge[],
  edgeStroke: string,
  accentStroke: string,
  groupBy: GroupBy,
  localNodeId: string | null,
): { rfNodes: RFNode[]; rfEdges: RFEdge[] } {
  if (groupBy === 'node') {
    return { rfNodes: toNodeGroupedFlowNodes(dgNodes, localNodeId), rfEdges: [] };
  }
  return {
    rfNodes: toReactFlowNodes(dgNodes, dgEdges),
    rfEdges: toReactFlowEdges(dgEdges, dgNodes, edgeStroke, accentStroke),
  };
}

/**
 * Manages layout computation and lightweight in-place updates.
 *
 * When the topology changes (nodes/edges added or removed), runs a full
 * layout: async ELK for the network view, the deterministic grid of
 * layout/nodeLayout.ts for the per-node view. When only data changes
 * (status, ports, stats, etc.), patches the existing positioned nodes/edges
 * without relayout — positions are kept, so role groups the user dragged
 * stay where they were dropped until the next topology change or view switch.
 *
 * `groupBy` picks the network view (default) or the per-swarm-node view;
 * `localNodeId` is the swarm node id of the local daemon, which hosts the
 * standalone containers in the per-node view.
 */
export function useGraphLayout(
  dgNodes: DGNode[],
  dgEdges: DGEdge[],
  edgeStroke: string,
  accentStroke: string,
  groupBy: GroupBy = 'network',
  localNodeId: string | null = null,
): GraphLayoutResult {
  const [nodes, setNodes, onNodesChange] = useNodesState<RFNode>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<RFEdge>([]);

  const topoKey = useMemo(
    () => topologyKey(dgNodes, dgEdges, groupBy, localNodeId),
    [dgNodes, dgEdges, groupBy, localNodeId],
  );
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
    let cancelled = false;

    let layoutPromise: Promise<{ nodes: RFNode[]; edges: RFEdge[] }>;
    if (dgNodes.length === 0) {
      // Empty topology (e.g. a stack with no resources): clear the canvas so
      // the empty state never sits over a stale, still-interactive graph.
      layoutPromise = Promise.resolve({ nodes: [], edges: [] });
    } else {
      const { rfNodes, rfEdges } = buildFlow(dgNodes, dgEdges, edgeStroke, accentStroke, groupBy, localNodeId);
      // The per-node view has a deterministic grid layout (no ELK, no edges).
      layoutPromise = groupBy === 'node'
        ? Promise.resolve({ nodes: layoutNodeGroups(rfNodes), edges: rfEdges })
        : computeLayout(rfNodes, rfEdges);
    }

    layoutPromise
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
  // lightweight update below without re-running ELK. groupBy and localNodeId
  // are folded into topoKey.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [topoKey, setNodes, setEdges]);

  // Lightweight update — apply status/style changes without relayout.
  // settledTopoKey is in deps so this re-runs after a fresh layout completes,
  // catching data that arrived while the layout was in flight.
  useEffect(() => {
    if (dgNodes.length === 0 || topoKey !== settledTopoKey) return;

    const { rfNodes, rfEdges } = buildFlow(dgNodes, dgEdges, edgeStroke, accentStroke, groupBy, localNodeId);
    const rfEdgeMap = new Map(rfEdges.map((e) => [e.id, e]));
    setEdges((prev) => prev.map((e) => {
      const updated = rfEdgeMap.get(e.id);
      return updated ? { ...e, data: { ...e.data, ...updated.data }, style: updated.style } : e;
    }));

    const rfNodeMap = new Map(rfNodes.map((n) => [n.id, n]));
    setNodes((prev) => prev.map((n) => {
      const updated = rfNodeMap.get(n.id);
      return updated ? { ...n, data: { ...n.data, ...updated.data } } : n;
    }));
  }, [dgNodes, dgEdges, edgeStroke, accentStroke, groupBy, localNodeId, topoKey, settledTopoKey, setNodes, setEdges]);

  return { nodes, edges, setNodes, setEdges, onNodesChange, onEdgesChange, layoutBusy, layoutError };
}
