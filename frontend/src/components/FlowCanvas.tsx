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
import { computeLayout } from '../layout/elk';
import { networkColor } from '../utils/colors';
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

function toReactFlowEdges(dfEdges: DFEdge[], dfNodes: DFNode[]): RFEdge[] {
  const nodeMap = new Map(dfNodes.map((n) => [n.id, n]));

  return dfEdges.map((e) => {
    const isVolume = e.type === 'volume_mount';
    const isSecondary = e.type === 'secondary_network';

    let stroke = '#475569';
    if (isVolume) stroke = '#f97316';
    if (isSecondary) {
      const targetNet = nodeMap.get(e.target);
      stroke = targetNet ? networkColor(targetNet.name) : '#475569';
    }

    return {
      id: e.id,
      source: e.source,
      target: e.target,
      type: 'elk',
      style: { stroke, strokeWidth: 1 },
    };
  });
}

export function FlowCanvas({ dfNodes, dfEdges, connected }: FlowCanvasProps) {
  const [nodes, setNodes, onNodesChange] = useNodesState<RFNode>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<RFEdge>([]);

  useEffect(() => {
    if (dfNodes.length === 0) return;
    let cancelled = false;

    const rfNodes = toReactFlowNodes(dfNodes, dfEdges);
    const rfEdges = toReactFlowEdges(dfEdges, dfNodes);

    computeLayout(rfNodes, rfEdges).then((layout) => {
      if (cancelled) return;
      setNodes(layout.nodes);
      setEdges(layout.edges);
    });

    return () => { cancelled = true; };
  }, [dfNodes, dfEdges, setNodes, setEdges]);

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
          background: '#1e293b',
          border: '1px solid #334155',
          borderRadius: 6,
          padding: '4px 10px',
          fontSize: 11,
          color: '#94a3b8',
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

      <ReactFlow
        nodes={nodes}
        edges={edges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        nodeTypes={nodeTypes}
        edgeTypes={edgeTypes}
        fitView
        proOptions={{ hideAttribution: true }}
        style={{ background: '#0f172a' }}
      >
        <Background variant={BackgroundVariant.Dots} color="#1e293b" gap={20} />
        <Controls
          style={{ background: '#1e293b', border: '1px solid #334155', borderRadius: 6 }}
        />
        <MiniMap
          style={{ background: '#1e293b', border: '1px solid #334155' }}
          maskColor="rgba(15, 23, 42, 0.7)"
          nodeColor={(node) => {
            if (node.type === 'networkGroup') {
              return networkColor((node.data as { dfNode: DFNode }).dfNode.name);
            }
            if (node.type === 'volumeNode') {
              return '#f97316';
            }
            return '#94a3b8';
          }}
        />
      </ReactFlow>
    </div>
  );
}
