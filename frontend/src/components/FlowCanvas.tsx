import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  ReactFlow,
  MiniMap,
  Controls,
  Background,
  BackgroundVariant,
  useNodesState,
  useEdgesState,
  type Node as RFNode,
  type Edge as RFEdge,
} from '@xyflow/react';
import { ANIMATION_NODE_LIMIT } from '../utils/constants';

import { ContainerNode } from './ContainerNode';
import { NetworkGroup } from './NetworkGroup';
import { VolumeNode } from './VolumeNode';
import { ElkEdge } from './ElkEdge';
import { CanvasEdgeLayer, type CanvasEdgeLayerHandle } from './CanvasEdgeLayer';
import { ThemeToggle } from './ThemeToggle';
import { StatusIndicator } from './StatusIndicator';
import { computeLayout } from '../layout/elk';
import { toReactFlowNodes, toReactFlowEdges } from '../utils/graphTransform';
import { useSelectionHighlight } from '../hooks/useSelectionHighlight';
import { networkColor } from '../utils/colors';
import { useTheme } from '../theme';
import type { DGNode, DGEdge } from '../types';

const nodeTypes = {
  containerNode: ContainerNode,
  networkGroup: NetworkGroup,
  volumeNode: VolumeNode,
};

const edgeTypes = {
  elk: ElkEdge,
};

interface FlowCanvasProps {
  dgNodes: DGNode[];
  dgEdges: DGEdge[];
  connected: boolean;
}

/**
 * Topology fingerprint — changes only when nodes or edges are added/removed.
 * Status changes (running → exited) don't alter the fingerprint, so they
 * skip the expensive ELK layout and only update node/edge data in place.
 */
function topologyKey(dgNodes: DGNode[], dgEdges: DGEdge[]): string {
  const nk = dgNodes.map((n) => n.id).sort().join(',');
  const ek = dgEdges.map((e) => e.id).sort().join(',');
  return nk + '|' + ek;
}

export function FlowCanvas({ dgNodes, dgEdges, connected }: FlowCanvasProps) {
  const [nodes, setNodes, onNodesChange] = useNodesState<RFNode>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<RFEdge>([]);
  const { theme } = useTheme();
  const canvasEdgeRef = useRef<CanvasEdgeLayerHandle>(null);

  const topoKey = useMemo(() => topologyKey(dgNodes, dgEdges), [dgNodes, dgEdges]);
  const prevTopoKeyRef = useRef('');
  const [layoutBusy, setLayoutBusy] = useState(false);
  const [layoutError, setLayoutError] = useState(false);

  // Full ELK layout — only when topology (node/edge set) changes.
  useEffect(() => {
    if (dgNodes.length === 0) return;
    let cancelled = false;

    setLayoutBusy(true);
    setLayoutError(false);
    const rfNodes = toReactFlowNodes(dgNodes, dgEdges);
    const rfEdges = toReactFlowEdges(dgEdges, dgNodes, theme.edgeStroke);

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
        if (!cancelled) setLayoutBusy(false);
      });

    return () => { cancelled = true; };
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [topoKey, setNodes, setEdges]);

  // Lightweight update — apply status/style changes without relayout.
  useEffect(() => {
    if (dgNodes.length === 0 || topoKey !== prevTopoKeyRef.current) return;

    const rfEdges = toReactFlowEdges(dgEdges, dgNodes, theme.edgeStroke);
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
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [dgNodes, dgEdges, theme.edgeStroke, setNodes, setEdges]);

  const showEmptyState = dgNodes.length === 0;
  const largeGraph = dgNodes.length > ANIMATION_NODE_LIMIT;

  const { styledNodes, styledEdges, canvasEdges, svgEdges, onNodeClick, onEdgeClick, onPaneClick } =
    useSelectionHighlight(nodes, edges, largeGraph);

  // Canvas edge hit-test helper — returns the edge if hit, null otherwise.
  const canvasEdgeHit = useCallback(
    (event: React.MouseEvent): RFEdge | null => {
      if (!largeGraph || !canvasEdgeRef.current) return null;
      const hitId = canvasEdgeRef.current.hitTest(event.clientX, event.clientY);
      if (!hitId) return null;
      return edges.find((e) => e.id === hitId) ?? null;
    },
    [largeGraph, edges],
  );

  // For large graphs, intercept node clicks to check if the click actually
  // hit a canvas edge passing through the node area (common with group nodes).
  const handleNodeClick = useCallback(
    (event: React.MouseEvent, node: RFNode) => {
      const edgeHit = canvasEdgeHit(event);
      if (edgeHit) {
        onEdgeClick(event, edgeHit);
      } else {
        onNodeClick(event, node);
      }
    },
    [canvasEdgeHit, onEdgeClick, onNodeClick],
  );

  // Pane click handler: test canvas edges first, then clear selection.
  const handlePaneClick = useCallback(
    (event: React.MouseEvent) => {
      const edgeHit = canvasEdgeHit(event);
      if (edgeHit) {
        onEdgeClick(event, edgeHit);
      } else {
        onPaneClick();
      }
    },
    [canvasEdgeHit, onEdgeClick, onPaneClick],
  );

  return (
    <div style={{ width: '100%', height: '100%', position: 'relative' }}>
      <StatusIndicator connected={connected} />
      <ThemeToggle />

      {showEmptyState && (
        <div
          style={{
            position: 'absolute',
            inset: 0,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            zIndex: 5,
            pointerEvents: 'none',
          }}
        >
          <p style={{ color: theme.nodeSubtext, fontSize: 14 }}>
            {connected
              ? 'No containers detected. Start a container to visualize the graph.'
              : 'Connecting to backend\u2026'}
          </p>
        </div>
      )}

      {layoutBusy && !showEmptyState && (
        <div
          style={{
            position: 'absolute',
            inset: 0,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            zIndex: 5,
            pointerEvents: 'none',
          }}
        >
          <p style={{ color: theme.nodeSubtext, fontSize: 14 }}>
            Computing layout for {dgNodes.length} containers\u2026
          </p>
        </div>
      )}

      {layoutError && !showEmptyState && (
        <div
          style={{
            position: 'absolute',
            inset: 0,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            zIndex: 5,
            pointerEvents: 'none',
          }}
        >
          <p style={{ color: theme.nodeSubtext, fontSize: 14 }}>
            Layout computation failed. Try reloading the page.
          </p>
        </div>
      )}

      {largeGraph && (
        <CanvasEdgeLayer ref={canvasEdgeRef} edges={canvasEdges} />
      )}

      <ReactFlow
        nodes={styledNodes}
        edges={largeGraph ? svgEdges : styledEdges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        onNodeClick={largeGraph ? handleNodeClick : onNodeClick}
        onEdgeClick={onEdgeClick}
        onPaneClick={handlePaneClick}
        nodeTypes={nodeTypes}
        edgeTypes={edgeTypes}
        nodesDraggable={false}
        nodesConnectable={false}
        onlyRenderVisibleElements
        fitView
        minZoom={0.05}
        maxZoom={2}
        style={{ background: theme.canvasBg }}
      >
        {!largeGraph && <Background variant={BackgroundVariant.Dots} color={theme.dotColor} gap={20} />}
        <Controls
          showInteractive={false}
          style={{ background: theme.panelBg, border: `1px solid ${theme.panelBorder}`, borderRadius: 6 }}
        />
        {!largeGraph && (
          <MiniMap
            style={{ background: theme.minimapBg, border: `1px solid ${theme.panelBorder}` }}
            maskColor={theme.minimapMask}
            nodeColor={(node) => {
              if (node.type === 'networkGroup') {
                return networkColor((node.data as { dgNode: DGNode }).dgNode.name) + '40';
              }
              if (node.type === 'volumeNode') {
                return '#f9731640';
              }
              return theme.nodeBorder;
            }}
          />
        )}
      </ReactFlow>
    </div>
  );
}
