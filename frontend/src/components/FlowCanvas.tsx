import { useCallback, useEffect, useState } from 'react';
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
import { computeLayout } from '../layout/elk';
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

const UNMANAGED_GROUP_ID = 'group:unmanaged';

function toReactFlowNodes(dfNodes: DFNode[], dfEdges: DFEdge[]): RFNode[] {
  const rfNodes: RFNode[] = [];

  const containers = dfNodes.filter((n) => n.type === 'container');
  const networks = dfNodes.filter((n) => n.type === 'network');

  const networksWithChildren = new Set(
    containers.filter((c) => c.networkId).map((c) => c.networkId),
  );
  const networksWithEdges = new Set(
    dfEdges.filter((e) => e.type === 'secondary_network').map((e) => e.target),
  );

  for (const net of networks) {
    if (!networksWithChildren.has(net.id) && !networksWithEdges.has(net.id)) {
      continue;
    }
    rfNodes.push({
      id: net.id,
      type: 'networkGroup',
      position: { x: 0, y: 0 },
      data: { dfNode: net },
      style: { width: 200, height: 150 },
    });
  }

  const hasUnmanaged = containers.some((c) => !c.source && !c.networkId);

  if (hasUnmanaged) {
    rfNodes.push({
      id: UNMANAGED_GROUP_ID,
      type: 'networkGroup',
      position: { x: 0, y: 0 },
      data: {
        dfNode: { id: UNMANAGED_GROUP_ID, type: 'network', name: 'unmanaged' } as DFNode,
      },
      style: { width: 200, height: 150 },
    });
  }

  for (const c of containers) {
    const node: RFNode = {
      id: c.id,
      type: 'containerNode',
      position: { x: 0, y: 0 },
      data: { dfNode: c },
    };
    if (c.networkId) {
      node.parentId = c.networkId;
      node.extent = 'parent';
    } else if (!c.source) {
      node.parentId = UNMANAGED_GROUP_ID;
      node.extent = 'parent';
    }
    rfNodes.push(node);
  }

  const containerGroup = new Map<string, string>();
  for (const c of containers) {
    if (c.networkId) {
      containerGroup.set(c.id, c.networkId);
    } else if (!c.source) {
      containerGroup.set(c.id, UNMANAGED_GROUP_ID);
    }
  }  const volumeMountEdges = dfEdges.filter((e) => e.type === 'volume_mount');
  const volumeGroupMap = new Map<string, string | null>();
  for (const e of volumeMountEdges) {
    const group = containerGroup.get(e.target);
    if (!volumeGroupMap.has(e.source)) {
      volumeGroupMap.set(e.source, group ?? null);
    } else {
      const existing = volumeGroupMap.get(e.source);
      if (existing !== group) {
        volumeGroupMap.set(e.source, null); // spans multiple groups
      }
    }
  }

  const groupIds = new Set(rfNodes.filter((n) => n.type === 'networkGroup').map((n) => n.id));

  const volumes = dfNodes.filter((n) => n.type === 'volume');
  for (const v of volumes) {
    const group = volumeGroupMap.get(v.id);
    const node: RFNode = {
      id: v.id,
      type: 'volumeNode',
      position: { x: 0, y: 0 },
      data: { dfNode: v },
    };
    if (group && groupIds.has(group)) {
      node.parentId = group;
      node.extent = 'parent';
    }
    rfNodes.push(node);
  }

  return rfNodes;
}

function toReactFlowEdges(dfEdges: DFEdge[], dfNodes: DFNode[], defaultStroke: string): RFEdge[] {
  const nodeMap = new Map(dfNodes.map((n) => [n.id, n]));

  return dfEdges.map((e) => {
    const isVolume = e.type === 'volume_mount';
    const isSecondary = e.type === 'secondary_network';

    let stroke = defaultStroke;
    if (isVolume) stroke = '#f97316';
    if (isSecondary) {
      const targetNet = nodeMap.get(e.target);
      stroke = targetNet ? networkColor(targetNet.name) : defaultStroke;
    }

    return {
      id: e.id,
      source: e.source,
      target: e.target,
      type: 'elk',
      data: { edgeType: e.type },
      style: { stroke, strokeWidth: 1 },
    };
  });
}

export function FlowCanvas({ dfNodes, dfEdges, connected }: FlowCanvasProps) {
  const [nodes, setNodes, onNodesChange] = useNodesState<RFNode>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<RFEdge>([]);
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null);
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

  const connectedEdgeIds = new Set<string>();
  const connectedNodeIds = new Set<string>();
  if (selectedNodeId) {
    connectedNodeIds.add(selectedNodeId);
    for (const e of edges) {
      if (e.source === selectedNodeId || e.target === selectedNodeId) {
        connectedEdgeIds.add(e.id);
        connectedNodeIds.add(e.source);
        connectedNodeIds.add(e.target);
      }
    }
  }

  const styledNodes = selectedNodeId
    ? nodes.map((n) => {
        // Don't dim network groups — they're containers, not selectable targets
        if (n.type === 'networkGroup') return n;
        const highlighted = connectedNodeIds.has(n.id);
        return { ...n, style: { ...n.style, opacity: highlighted ? 1 : 0.2 } };
      })
    : nodes;

  const styledEdges = selectedNodeId
    ? edges.map((e) => {
        const highlighted = connectedEdgeIds.has(e.id);
        return {
          ...e,
          style: { ...e.style, opacity: highlighted ? 1 : 0.15 },
        };
      })
    : edges;

  const onNodeClick = useCallback((_: React.MouseEvent, node: RFNode) => {
    if (node.type === 'networkGroup') return;
    setSelectedNodeId((prev) => (prev === node.id ? null : node.id));
  }, []);

  const onPaneClick = useCallback(() => {
    setSelectedNodeId(null);
  }, []);

  return (
    <div style={{ width: '100%', height: '100%', position: 'relative' }}>
      <div
        style={{
          position: 'absolute',
          top: 10,
          left: 10,
          zIndex: 10,
          display: 'flex',
          alignItems: 'center',
          gap: 6,
          background: theme.panelBg,
          border: `1px solid ${theme.panelBorder}`,
          borderRadius: 6,
          padding: '4px 10px',
          fontSize: 11,
          color: theme.panelText,
        }}
      >
        <span
          style={{
            width: 7,
            height: 7,
            borderRadius: '50%',
            background: connected ? '#22c55e' : '#ef4444',
          }}
        />
        {connected ? 'Live' : 'Disconnected'}
      </div>

      <ThemeToggle />

      <ReactFlow
        nodes={styledNodes}
        edges={styledEdges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        onNodeClick={onNodeClick}
        onPaneClick={onPaneClick}
        nodeTypes={nodeTypes}
        edgeTypes={edgeTypes}
        nodesDraggable={false}
        nodesConnectable={false}
        fitView
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
