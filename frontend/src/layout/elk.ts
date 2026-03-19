import ELK, { type ElkNode, type ElkExtendedEdge } from 'elkjs/lib/elk.bundled';
import type { Node as RFNode, Edge as RFEdge } from '@xyflow/react';

const elk = new ELK();

const NODE_WIDTH = 200;
const NODE_HEIGHT = 65;
const VOLUME_HEIGHT = 40;

const ELK_OPTIONS = {
  'elk.algorithm': 'layered',
  'elk.direction': 'DOWN',
  'elk.hierarchyHandling': 'INCLUDE_CHILDREN',
  'elk.edgeRouting': 'ORTHOGONAL',
  'elk.spacing.nodeNode': '20',
  'elk.spacing.edgeNode': '20',
  'elk.spacing.edgeEdge': '12',
  'elk.layered.spacing.nodeNodeBetweenLayers': '35',
  'elk.spacing.componentComponent': '60',
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
  const allEdgePaths = new Map<string, string>();  const allElkChildren: ElkNode[] = [];
  const allRootEdges: ElkExtendedEdge[] = [];

  for (const compIds of components) {
    const compIdSet = new Set(compIds);

    for (const id of compIds) {
      const rfNode = nodeMap.get(id);
      if (!rfNode) continue;

      if (rfNode.type === 'networkGroup') {
        const groupChildren = children.filter((n) => n.parentId === id);
        allElkChildren.push({
          id,
          layoutOptions: GROUP_OPTIONS,
          children: groupChildren.length > 0
            ? groupChildren.map((child) => ({
                id: child.id,
                width: NODE_WIDTH,
                height: nodeMap.get(child.id)?.type === 'volumeNode' ? VOLUME_HEIGHT : NODE_HEIGHT,
                layoutOptions: { 'elk.alignment': 'TOP' },
              }))
            : [{ id: `${id}__placeholder`, width: 120, height: 1 }],
        });
      } else {
        const h = rfNode.type === 'volumeNode' ? VOLUME_HEIGHT : NODE_HEIGHT;
        allElkChildren.push({ id, width: NODE_WIDTH, height: h, layoutOptions: { 'elk.alignment': 'TOP' } });
      }
    }

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
        allRootEdges.push(elkEdge);
      }
    }

    for (const elkChild of allElkChildren) {
      const groupEdges = groupEdgeMap.get(elkChild.id);
      if (groupEdges) {
        (elkChild as ElkNode & { edges: ElkExtendedEdge[] }).edges = groupEdges;
      }
    }
  }  const wrappedChildren: ElkNode[] = [];
  const wrappedEdges: ElkExtendedEdge[] = [];

  for (const compIds of components) {
    const compIdSet = new Set(compIds);
    const compChildren = allElkChildren.filter((c) => compIdSet.has(c.id));
    const compEdges = allRootEdges.filter((e) => {
      const s = childToParent.get(e.sources[0]) ?? e.sources[0];
      const t = childToParent.get(e.targets[0]) ?? e.targets[0];
      return compIdSet.has(s) && compIdSet.has(t);
    });

    if (compChildren.length === 1 && compEdges.length === 0) {
      // Single node, no need to wrap
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

  const elkGraph: ElkNode = {
    id: 'root',
    layoutOptions: {
      'elk.algorithm': 'layered',
      'elk.direction': 'DOWN',
      'elk.edgeRouting': 'ORTHOGONAL',
      'elk.separateConnectedComponents': 'true',
      'elk.spacing.nodeNode': '40',
      'elk.aspectRatio': '1.4',
      'elk.padding': '[top=0,left=0,bottom=0,right=0]',
    },
    children: wrappedChildren,
    edges: wrappedEdges,
  };

  const layout = await elk.layout(elkGraph);

  for (const topNode of layout.children ?? []) {
    const isWrapper = topNode.id.startsWith('__comp_');

    if (isWrapper) {
      const ox = topNode.x ?? 0;
      const oy = topNode.y ?? 0;

      for (const elkNode of topNode.children ?? []) {
        const rfNode = nodeMap.get(elkNode.id);
        if (!rfNode) continue;

        rfNode.position = {
          x: (elkNode.x ?? 0) + ox,
          y: (elkNode.y ?? 0) + oy,
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
    } else {
      const rfNode = nodeMap.get(topNode.id);
      if (rfNode) {
        rfNode.position = {
          x: topNode.x ?? 0,
          y: topNode.y ?? 0,
        };

        if (rfNode.type === 'networkGroup') {
          rfNode.style = {
            ...rfNode.style,
            width: topNode.width,
            height: topNode.height,
          };
        }

        for (const elkChild of topNode.children ?? []) {
          const rfChild = nodeMap.get(elkChild.id);
          if (rfChild) {
            rfChild.position = { x: elkChild.x ?? 0, y: elkChild.y ?? 0 };
          }
        }
      }
    }
  }

  extractEdgePaths(layout, 0, 0, allEdgePaths);

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
