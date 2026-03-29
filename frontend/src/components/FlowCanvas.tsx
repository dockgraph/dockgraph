import { useCallback, useRef } from 'react';
import {
  ReactFlow,
  MiniMap,
  Controls,
  Background,
  BackgroundVariant,
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
import { useGraphLayout } from '../hooks/useGraphLayout';
import { useSelectionHighlight } from '../hooks/useSelectionHighlight';
import { networkColor } from '../utils/colors';
import { useTheme } from '../theme';
import type { DGNode, DGEdge } from '../types';
import type { ReactNode } from 'react';

function Overlay({ children }: { children: ReactNode }) {
  return (
    <div
      role="status"
      aria-live="polite"
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
      {children}
    </div>
  );
}

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

export function FlowCanvas({ dgNodes, dgEdges, connected }: FlowCanvasProps) {
  const { theme } = useTheme();
  const canvasEdgeRef = useRef<CanvasEdgeLayerHandle>(null);

  const { nodes, edges, onNodesChange, onEdgesChange, layoutBusy, layoutError } =
    useGraphLayout(dgNodes, dgEdges, theme.edgeStroke);

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
        <Overlay>
          <p style={{ color: theme.nodeSubtext, fontSize: 14 }}>
            {connected
              ? 'No containers detected. Start a container to visualize the graph.'
              : 'Connecting to backend…'}
          </p>
        </Overlay>
      )}

      {layoutBusy && !showEmptyState && (
        <Overlay>
          <p style={{ color: theme.nodeSubtext, fontSize: 14 }}>
            Computing layout for {dgNodes.length} nodes…
          </p>
        </Overlay>
      )}

      {layoutError && !showEmptyState && (
        <Overlay>
          <p style={{ color: theme.nodeSubtext, fontSize: 14 }}>
            Layout computation failed. Try reloading the page.
          </p>
        </Overlay>
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
