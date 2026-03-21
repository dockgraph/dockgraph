import type { ElkNode, ElkExtendedEdge } from 'elkjs/lib/elk.bundled';
import type { Node as RFNode, Edge as RFEdge } from '@xyflow/react';
import { CONTAINER_NODE_HEIGHT, VOLUME_NODE_HEIGHT } from '../utils/constants';

/** Padding for network group nodes. */
const GROUP_OPTIONS = {
  'elk.padding': '[top=35,left=15,bottom=12,right=15]',
};

/** ELK layout options for the inner component graphs. */
export const ELK_OPTIONS = {
  'elk.algorithm': 'layered',
  'elk.direction': 'DOWN',
  'elk.hierarchyHandling': 'INCLUDE_CHILDREN',
  'elk.edgeRouting': 'ORTHOGONAL',
  'elk.spacing.nodeNode': '20',
  'elk.spacing.edgeNode': '20',
  'elk.spacing.edgeEdge': '12',
  'elk.layered.spacing.nodeNodeBetweenLayers': '35',
  'elk.layered.spacing.edgeNodeBetweenLayers': '15',
  'elk.layered.spacing.edgeEdgeBetweenLayers': '12',
  'elk.spacing.componentComponent': '60',
  'elk.layered.crossingMinimization.strategy': 'LAYER_SWEEP',
  'elk.layered.nodePlacement.strategy': 'NETWORK_SIMPLEX',
  'elk.layered.nodePlacement.networkSimplex.nodeFlexibility.default': 'NODE_SIZE',
  'elk.layered.compaction.postCompaction.strategy': 'NONE',
};

export interface ClassifiedNodes {
  groups: RFNode[];
  children: RFNode[];
  freeNodes: RFNode[];
  childToParent: Map<string, string>;
}

/** Splits React Flow nodes by role: groups, children (parented), and free-standing. */
export function classifyNodes(nodes: RFNode[]): ClassifiedNodes {
  const groups = nodes.filter((n) => n.type === 'networkGroup');
  const children = nodes.filter((n) => n.parentId);
  const freeNodes = nodes.filter((n) => n.type !== 'networkGroup' && !n.parentId);

  const childToParent = new Map<string, string>();
  for (const c of children) {
    if (c.parentId) childToParent.set(c.id, c.parentId);
  }

  return { groups, children, freeNodes, childToParent };
}

function nodeHeight(rfNode: RFNode): number {
  return rfNode.type === 'volumeNode' ? VOLUME_NODE_HEIGHT : CONTAINER_NODE_HEIGHT;
}

/**
 * Builds the ELK hierarchy nodes and partitions edges into group-internal
 * and root-level. Each connected component is processed independently.
 */
export function buildElkChildren(
  components: string[][],
  nodeMap: Map<string, RFNode>,
  children: RFNode[],
  edges: RFEdge[],
  childToParent: Map<string, string>,
  nodeWidth: number,
): { elkChildren: ElkNode[]; rootEdges: ElkExtendedEdge[] } {
  const elkChildren: ElkNode[] = [];
  const rootEdges: ElkExtendedEdge[] = [];

  for (const compIds of components) {
    const compIdSet = new Set(compIds);

    for (const id of compIds) {
      const rfNode = nodeMap.get(id);
      if (!rfNode) continue;

      if (rfNode.type === 'networkGroup') {
        const groupChildren = children.filter((n) => n.parentId === id);
        elkChildren.push({
          id,
          layoutOptions: GROUP_OPTIONS,
          children: groupChildren.length > 0
            ? groupChildren.map((child) => ({
                id: child.id,
                width: nodeWidth,
                height: nodeHeight(nodeMap.get(child.id) ?? child),
                layoutOptions: { 'elk.alignment': 'TOP' },
              }))
            : [{ id: `${id}__placeholder`, width: 120, height: 1 }],
        });
      } else {
        elkChildren.push({
          id,
          width: nodeWidth,
          height: nodeHeight(rfNode),
          layoutOptions: { 'elk.alignment': 'TOP' },
        });
      }
    }

    // Partition edges: same-group edges attach to the group node, cross-group
    // edges go to the root level for proper hierarchical routing.
    const groupEdgeMap = new Map<string, ElkExtendedEdge[]>();

    for (const e of edges) {
      const sTop = childToParent.get(e.source) ?? e.source;
      const tTop = childToParent.get(e.target) ?? e.target;
      if (!compIdSet.has(sTop) || !compIdSet.has(tTop)) continue;

      const elkEdge: ElkExtendedEdge = { id: e.id, sources: [e.source], targets: [e.target] };

      const sGroup = childToParent.get(e.source);
      const tGroup = childToParent.get(e.target);
      if (sGroup && tGroup && sGroup === tGroup) {
        const list = groupEdgeMap.get(sGroup) ?? [];
        list.push(elkEdge);
        groupEdgeMap.set(sGroup, list);
      } else {
        rootEdges.push(elkEdge);
      }
    }

    for (const elkChild of elkChildren) {
      const groupEdges = groupEdgeMap.get(elkChild.id);
      if (groupEdges) {
        (elkChild as ElkNode & { edges: ElkExtendedEdge[] }).edges = groupEdges;
      }
    }
  }

  return { elkChildren, rootEdges };
}

/**
 * Wraps connected components into either standalone nodes or component
 * wrapper nodes for the root-level ELK graph. Single-node components
 * without edges skip the wrapper to avoid unnecessary padding.
 */
export function wrapComponents(
  components: string[][],
  elkChildren: ElkNode[],
  rootEdges: ElkExtendedEdge[],
  childToParent: Map<string, string>,
): { wrappedChildren: ElkNode[]; wrappedEdges: ElkExtendedEdge[] } {
  const wrappedChildren: ElkNode[] = [];
  const wrappedEdges: ElkExtendedEdge[] = [];

  for (const compIds of components) {
    const compIdSet = new Set(compIds);
    const compChildren = elkChildren.filter((c) => compIdSet.has(c.id));
    const compEdges = rootEdges.filter((e) => {
      const s = childToParent.get(e.sources[0]) ?? e.sources[0];
      const t = childToParent.get(e.targets[0]) ?? e.targets[0];
      return compIdSet.has(s) && compIdSet.has(t);
    });

    if (compChildren.length === 1 && compEdges.length === 0) {
      wrappedChildren.push(compChildren[0]);
    } else {
      wrappedChildren.push({
        id: `__comp_${compIds[0]}`,
        layoutOptions: {
          ...ELK_OPTIONS,
          'elk.hierarchyHandling': 'INCLUDE_CHILDREN',
          'elk.padding': '[top=0,left=0,bottom=0,right=0]',
        },
        children: compChildren,
        edges: compEdges,
      });
    }
  }

  return { wrappedChildren, wrappedEdges };
}
