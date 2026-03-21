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
import type { DFNode, DFEdge } from '../types';

const nodeTypes = {
  containerNode: ContainerNode,
  networkGroup: NetworkGroup,
  volumeNode: VolumeNode,
};

const edgeTypes = {
  elk: ElkEdge,
};

interface FlowCanvasProps {
  dfNodes: DFNode[];
  dfEdges: DFEdge[];
  connected: boolean;
}

export function FlowCanvas({ dfNodes, dfEdges, connected }: FlowCanvasProps) {
  const [nodes, setNodes, onNodesChange] = useNodesState<RFNode>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<RFEdge>([]);
  const { theme } = useTheme();

  useEffect(() => {
    if (dfNodes.length === 0) return;
    let cancelled = false;

    const rfNodes = toReactFlowNodes(dfNodes, dfEdges);
    const rfEdges = toReactFlowEdges(dfEdges, dfNodes, theme.edgeStroke);

    computeLayout(rfNodes, rfEdges).then((layout) => {
      if (cancelled) return;
      setNodes(layout.nodes);
      setEdges(layout.edges);
    });

    return () => { cancelled = true; };
  }, [dfNodes, dfEdges, setNodes, setEdges, theme.edgeStroke]);

  const { styledNodes, styledEdges, onNodeClick, onEdgeClick, onPaneClick } =
    useSelectionHighlight(nodes, edges);

  return (
    <div style={{ width: '100%', height: '100%', position: 'relative' }}>
      <StatusIndicator connected={connected} />
      <ThemeToggle />

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
              return networkColor((node.data as { dfNode: DFNode }).dfNode.name) + '40';
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
