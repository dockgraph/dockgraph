import { useMemo } from 'react';
import {
  ReactFlow,
  MiniMap,
  Controls,
  Background,
  BackgroundVariant,
  type Node as RFNode,
  type Edge as RFEdge,
} from '@xyflow/react';

import { ContainerNode } from './ContainerNode';
import { NetworkGroup } from './NetworkGroup';
import { VolumeNode } from './VolumeNode';
import { computeLayout } from '../layout/dagre';
import { networkColor } from '../utils/colors';
import type { DFNode, DFEdge } from '../types';

const nodeTypes = {
  containerNode: ContainerNode,
  networkGroup: NetworkGroup,
  volumeNode: VolumeNode,
};

interface FlowCanvasProps {
  dfNodes: DFNode[];
  dfEdges: DFEdge[];
  connected: boolean;
}

const UNMANAGED_GROUP_ID = 'group:unmanaged';

function toReactFlowNodes(dfNodes: DFNode[]): RFNode[] {
  const rfNodes: RFNode[] = [];

  const networks = dfNodes.filter((n) => n.type === 'network');
  for (const net of networks) {
    rfNodes.push({
      id: net.id,
      type: 'networkGroup',
      position: { x: 0, y: 0 },
      data: { dfNode: net },
      style: { width: 200, height: 150 },
    });
  }

  const containers = dfNodes.filter((n) => n.type === 'container');
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

  const volumes = dfNodes.filter((n) => n.type === 'volume');
  for (const v of volumes) {
    rfNodes.push({
      id: v.id,
      type: 'volumeNode',
      position: { x: 0, y: 0 },
      data: { dfNode: v },
    });
  }

  return rfNodes;
}

function toReactFlowEdges(dfEdges: DFEdge[]): RFEdge[] {
  return dfEdges.map((e) => {
    const isVolume = e.type === 'volume_mount';
    const isSecondary = e.type === 'secondary_network';

    return {
      id: e.id,
      source: e.source,
      target: e.target,
      type: 'default',
      style: {
        stroke: isVolume ? '#f97316' : isSecondary ? '#64748b' : '#94a3b8',
        strokeDasharray: isVolume || isSecondary ? '5 3' : undefined,
      },
      animated: e.type === 'depends_on',
    };
  });
}

export function FlowCanvas({ dfNodes, dfEdges, connected }: FlowCanvasProps) {
  const { nodes, edges } = useMemo(() => {
    const rfNodes = toReactFlowNodes(dfNodes);
    const rfEdges = toReactFlowEdges(dfEdges);
    return computeLayout(rfNodes, rfEdges);
  }, [dfNodes, dfEdges]);

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
        nodeTypes={nodeTypes}
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
            return '#334155';
          }}
        />
      </ReactFlow>
    </div>
  );
}
