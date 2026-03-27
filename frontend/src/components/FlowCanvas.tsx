import { useEffect } from 'react';
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

import { ContainerNode } from './ContainerNode';
import { NetworkGroup } from './NetworkGroup';
import { VolumeNode } from './VolumeNode';
import { ElkEdge } from './ElkEdge';
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

export function FlowCanvas({ dgNodes, dgEdges, connected }: FlowCanvasProps) {
  const [nodes, setNodes, onNodesChange] = useNodesState<RFNode>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<RFEdge>([]);
  const { theme } = useTheme();

  useEffect(() => {
    if (dgNodes.length === 0) return;
    let cancelled = false;

    const rfNodes = toReactFlowNodes(dgNodes, dgEdges);
    const rfEdges = toReactFlowEdges(dgEdges, dgNodes, theme.edgeStroke);

    computeLayout(rfNodes, rfEdges)
      .then((layout) => {
        if (cancelled) return;
        setNodes(layout.nodes);
        setEdges(layout.edges);
      })
      .catch((err) => {
        console.error('layout computation failed:', err);
      });

    return () => { cancelled = true; };
  }, [dgNodes, dgEdges, setNodes, setEdges, theme.edgeStroke]);

  const { styledNodes, styledEdges, onNodeClick, onEdgeClick, onPaneClick } =
    useSelectionHighlight(nodes, edges);

  const showEmptyState = dgNodes.length === 0;

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

      <ReactFlow
        nodes={styledNodes}
        edges={styledEdges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        onNodeClick={onNodeClick}
        onEdgeClick={onEdgeClick}
        onPaneClick={onPaneClick}
        nodeTypes={nodeTypes}
        edgeTypes={edgeTypes}
        nodesDraggable={false}
        nodesConnectable={false}
        fitView
        minZoom={0.05}
        maxZoom={2}
        proOptions={{ hideAttribution: true }}
        style={{ background: theme.canvasBg }}
      >
        <Background variant={BackgroundVariant.Dots} color={theme.dotColor} gap={20} />
        <Controls
          showInteractive={false}
          style={{ background: theme.panelBg, border: `1px solid ${theme.panelBorder}`, borderRadius: 6 }}
        />
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
      </ReactFlow>
    </div>
  );
}
