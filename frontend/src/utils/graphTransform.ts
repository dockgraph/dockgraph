import type { Node as RFNode, Edge as RFEdge } from '@xyflow/react';
import { networkColor } from './colors';
import type { DGNode, DGEdge } from '../types';

const UNMANAGED_GROUP_ID = 'group:unmanaged';

const RUNNING_STATUSES = new Set(['running']);

function isEndpointActive(node: DGNode | undefined): boolean {
  if (!node) return false;
  if (node.type !== 'container') return true;
  return RUNNING_STATUSES.has(node.status ?? '');
}

/** Creates React Flow group nodes for networks that have children or secondary edges. */
function buildNetworkGroups(
  networks: DGNode[],
  containers: DGNode[],
  dgEdges: DGEdge[],
): RFNode[] {
  const rfNodes: RFNode[] = [];

  const networksWithChildren = new Set(
    containers.filter((c) => c.networkId).map((c) => c.networkId),
  );
  const networksWithEdges = new Set(
    dgEdges.filter((e) => e.type === 'secondary_network').map((e) => e.target),
  );

  for (const net of networks) {
    if (!networksWithChildren.has(net.id) && !networksWithEdges.has(net.id)) {
      continue;
    }
    rfNodes.push({
      id: net.id,
      type: 'networkGroup',
      position: { x: 0, y: 0 },
      data: { dgNode: net },
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
        dgNode: { id: UNMANAGED_GROUP_ID, type: 'network', name: 'unmanaged' } as DGNode,
      },
      style: { width: 200, height: 150 },
    });
  }

  return rfNodes;
}

/** Creates React Flow nodes for containers, assigning each to its network group. */
function buildContainerNodes(containers: DGNode[]): RFNode[] {
  return containers.map((c) => {
    const node: RFNode = {
      id: c.id,
      type: 'containerNode',
      position: { x: 0, y: 0 },
      data: { dgNode: c },
    };
    if (c.networkId) {
      node.parentId = c.networkId;
      node.extent = 'parent';
    } else if (!c.source) {
      node.parentId = UNMANAGED_GROUP_ID;
      node.extent = 'parent';
    }
    return node;
  });
}

/**
 * Creates React Flow nodes for volumes, placing each inside the network group
 * of its first consumer so volume-mount edges stay at the same hierarchy depth.
 */
function buildVolumeNodes(
  volumes: DGNode[],
  containers: DGNode[],
  dgEdges: DGEdge[],
  groupIds: Set<string>,
): RFNode[] {
  const containerGroup = new Map<string, string>();
  for (const c of containers) {
    if (c.networkId) {
      containerGroup.set(c.id, c.networkId);
    } else if (!c.source) {
      containerGroup.set(c.id, UNMANAGED_GROUP_ID);
    }
  }

  const volumeGroupMap = new Map<string, string>();
  for (const e of dgEdges.filter((e) => e.type === 'volume_mount')) {
    const group = containerGroup.get(e.target);
    if (!volumeGroupMap.has(e.source) && group) {
      volumeGroupMap.set(e.source, group);
    }
  }

  return volumes.map((v) => {
    const group = volumeGroupMap.get(v.id);
    const node: RFNode = {
      id: v.id,
      type: 'volumeNode',
      position: { x: 0, y: 0 },
      data: { dgNode: v },
    };
    if (group && groupIds.has(group)) {
      node.parentId = group;
      node.extent = 'parent';
    }
    return node;
  });
}

/**
 * Converts domain nodes (DGNode) into React Flow nodes, organizing containers
 * into network groups and placing volumes inside the group of their first consumer.
 *
 * The grouping ensures that ELK can route edges correctly — both endpoints of an
 * edge must be at the same hierarchy depth for proper cross-group edge routing.
 */
export function toReactFlowNodes(dgNodes: DGNode[], dgEdges: DGEdge[]): RFNode[] {
  const containers = dgNodes.filter((n) => n.type === 'container');
  const networks = dgNodes.filter((n) => n.type === 'network');
  const volumes = dgNodes.filter((n) => n.type === 'volume');

  const groups = buildNetworkGroups(networks, containers, dgEdges);
  const containerNodes = buildContainerNodes(containers);
  const groupIds = new Set(groups.map((n) => n.id));
  const volumeNodes = buildVolumeNodes(volumes, containers, dgEdges, groupIds);

  return [...groups, ...containerNodes, ...volumeNodes];
}

/**
 * Converts domain edges (DGEdge) into React Flow edges with appropriate
 * stroke colors and active/inactive state based on endpoint container status.
 */
export function toReactFlowEdges(dgEdges: DGEdge[], dgNodes: DGNode[], defaultStroke: string): RFEdge[] {
  const nodeMap = new Map(dgNodes.map((n) => [n.id, n]));

  return dgEdges.filter((e) => nodeMap.has(e.source) && nodeMap.has(e.target)).map((e) => {
    const isVolume = e.type === 'volume_mount';
    const isSecondary = e.type === 'secondary_network';

    let stroke = defaultStroke;
    if (isVolume) stroke = '#f97316';
    if (isSecondary) {
      const targetNet = nodeMap.get(e.target);
      stroke = targetNet ? networkColor(targetNet.name) : defaultStroke;
    }

    const sourceNode = nodeMap.get(e.source);
    const targetNode = nodeMap.get(e.target);
    const active = isEndpointActive(sourceNode) && isEndpointActive(targetNode);

    return {
      id: e.id,
      source: e.source,
      target: e.target,
      type: 'elk',
      data: { edgeType: e.type, active },
      style: { stroke, strokeWidth: 1 },
    };
  });
}
