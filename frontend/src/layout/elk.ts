import ELK, { type ElkNode, type ElkExtendedEdge } from 'elkjs/lib/elk.bundled';
import type { Node as RFNode, Edge as RFEdge } from '@xyflow/react';

const elk = new ELK();

const NODE_WIDTH = 220;
const NODE_HEIGHT = 65;
const VOLUME_HEIGHT = 40;
const COMPONENT_GAP = 60;

const ELK_OPTIONS = {
  'elk.algorithm': 'layered',
  'elk.direction': 'DOWN',
  'elk.hierarchyHandling': 'INCLUDE_CHILDREN',
  'elk.edgeRouting': 'ORTHOGONAL',
  'elk.spacing.nodeNode': '20',
  'elk.spacing.edgeNode': '20',
  'elk.spacing.edgeEdge': '12',
  'elk.layered.spacing.nodeNodeBetweenLayers': '35',
  'elk.spacing.componentComponent': '40',
  'elk.layered.crossingMinimization.strategy': 'LAYER_SWEEP',
  'elk.layered.nodePlacement.strategy': 'NETWORK_SIMPLEX',
  'elk.layered.compaction.postCompaction.strategy': 'EDGE_LENGTH',
};

const GROUP_OPTIONS = {
  'elk.padding': '[top=35,left=15,bottom=12,right=15]',
};

interface LayoutResult {
  nodes: RFNode[];
  edges: RFEdge[];
}

function findComponents(
  topIds: string[],
  edges: RFEdge[],
  childToParent: Map<string, string>,
): string[][] {
  function topOf(id: string): string | undefined {
    return childToParent.get(id) ?? (topIds.includes(id) ? id : undefined);
  }

  const adj = new Map<string, Set<string>>();
  for (const id of topIds) adj.set(id, new Set());

  for (const e of edges) {
    const s = topOf(e.source);
    const t = topOf(e.target);
    if (s && t && s !== t) {
      adj.get(s)?.add(t);
      adj.get(t)?.add(s);
    }
  }

  const visited = new Set<string>();
  const components: string[][] = [];

  for (const id of topIds) {
    if (visited.has(id)) continue;
    const component: string[] = [];
    const stack = [id];
    while (stack.length > 0) {
      const cur = stack.pop()!;
      if (visited.has(cur)) continue;
      visited.add(cur);
      component.push(cur);
      for (const neighbor of adj.get(cur) ?? []) {
        if (!visited.has(neighbor)) stack.push(neighbor);
      }
    }
    components.push(component);
  }

  return components;
}

export async function computeLayout(
  nodes: RFNode[],
  edges: RFEdge[],
): Promise<LayoutResult> {
  const groups = nodes.filter((n) => n.type === 'networkGroup');
  const children = nodes.filter((n) => n.parentId);
  const freeNodes = nodes.filter(
    (n) => n.type !== 'networkGroup' && !n.parentId,
  );

  const childToParent = new Map<string, string>();
  for (const c of children) {
    if (c.parentId) childToParent.set(c.id, c.parentId);
  }

  const topIds = [...groups.map((g) => g.id), ...freeNodes.map((n) => n.id)];
  const components = findComponents(topIds, edges, childToParent);

  const nodeMap = new Map(nodes.map((n) => [n.id, n]));
  const allEdgePaths = new Map<string, string>();
  let currentY = 0;

  for (const compIds of components) {
    const compIdSet = new Set(compIds);

    const elkChildren: ElkNode[] = [];
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
                width: NODE_WIDTH,
                height: nodeMap.get(child.id)?.type === 'volumeNode' ? VOLUME_HEIGHT : NODE_HEIGHT,
              }))
            : [{ id: `${id}__placeholder`, width: 1, height: 1 }],
        });
      } else {
        const h = rfNode.type === 'volumeNode' ? VOLUME_HEIGHT : NODE_HEIGHT;
        elkChildren.push({ id, width: NODE_WIDTH, height: h });
      }
    }

    // ELK uses group-local coordinates for within-group edge sections
    // but root coordinates for cross-hierarchy ones. Placing them at the
    // correct hierarchy level ensures extractEdgePaths applies the right offset.
    const groupEdgeMap = new Map<string, ElkExtendedEdge[]>();
    const rootEdges: ElkExtendedEdge[] = [];

    for (const e of edges) {
      const sTop = childToParent.get(e.source) ?? e.source;
      const tTop = childToParent.get(e.target) ?? e.target;
      if (!compIdSet.has(sTop) || !compIdSet.has(tTop)) continue;

      const elkEdge: ElkExtendedEdge = { id: e.id, sources: [e.source], targets: [e.target] };

      // Both endpoints in the same group → attach to that group
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

    // Attach within-group edges to their group nodes
    for (const elkChild of elkChildren) {
      const groupEdges = groupEdgeMap.get(elkChild.id);
      if (groupEdges) {
        (elkChild as ElkNode & { edges: ElkExtendedEdge[] }).edges = groupEdges;
      }
    }

    const elkGraph: ElkNode = {
      id: `comp-${compIds[0]}`,
      layoutOptions: ELK_OPTIONS,
      children: elkChildren,
      edges: rootEdges,
    };

    const layout = await elk.layout(elkGraph);

    for (const elkNode of layout.children ?? []) {
      const rfNode = nodeMap.get(elkNode.id);
      if (!rfNode) continue;

      rfNode.position = {
        x: elkNode.x ?? 0,
        y: (elkNode.y ?? 0) + currentY,
      };

      if (rfNode.type === 'networkGroup') {
        rfNode.style = {
          ...rfNode.style,
          width: elkNode.width,
          height: elkNode.height,
        };
      }

      for (const elkChild of elkNode.children ?? []) {
        const rfChild = nodeMap.get(elkChild.id);
        if (rfChild) {
          rfChild.position = { x: elkChild.x ?? 0, y: elkChild.y ?? 0 };
        }
      }
    }

    extractEdgePaths(layout, 0, currentY, allEdgePaths);
    currentY += (layout.height ?? 0) + COMPONENT_GAP;
  }

  const updatedEdges = edges.map((e) => {
    const path = allEdgePaths.get(e.id);
    if (path) {
      return { ...e, type: 'elk', data: { ...(e.data ?? {}), path } };
    }
    return e;
  });

  return { nodes: [...groups, ...children, ...freeNodes], edges: updatedEdges };
}

function extractEdgePaths(
  node: ElkNode,
  offsetX: number,
  offsetY: number,
  out: Map<string, string>,
): void {
  for (const edge of (node as { edges?: ElkExtendedEdge[] }).edges ?? []) {
    const section = edge.sections?.[0];
    if (!section) continue;

    let d = `M ${section.startPoint.x + offsetX} ${section.startPoint.y + offsetY}`;
    for (const bp of section.bendPoints ?? []) {
      d += ` L ${bp.x + offsetX} ${bp.y + offsetY}`;
    }
    d += ` L ${section.endPoint.x + offsetX} ${section.endPoint.y + offsetY}`;
    out.set(edge.id, d);
  }

  for (const child of node.children ?? []) {
    extractEdgePaths(
      child,
      offsetX + (child.x ?? 0),
      offsetY + (child.y ?? 0),
      out,
    );
  }
}
