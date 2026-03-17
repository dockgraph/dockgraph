import dagre from '@dagrejs/dagre';
import type { Node as RFNode, Edge as RFEdge } from '@xyflow/react';

const NODE_WIDTH = 150;
const NODE_HEIGHT = 60;
const GROUP_PADDING = 50;

interface LayoutResult {
  nodes: RFNode[];
  edges: RFEdge[];
}

export function computeLayout(nodes: RFNode[], edges: RFEdge[]): LayoutResult {
  const groups = nodes.filter((n) => n.type === 'networkGroup');
  const children = nodes.filter((n) => n.parentId);
  const freeNodes = nodes.filter((n) => n.type !== 'networkGroup' && !n.parentId);

  // Pass 1: Layout children within each group
  const groupSizes = new Map<string, { width: number; height: number }>();

  for (const group of groups) {
    const groupChildren = children.filter((n) => n.parentId === group.id);
    const groupEdges = edges.filter(
      (e) =>
        groupChildren.some((n) => n.id === e.source) &&
        groupChildren.some((n) => n.id === e.target),
    );

    if (groupChildren.length === 0) {
      groupSizes.set(group.id, {
        width: NODE_WIDTH + GROUP_PADDING * 2,
        height: NODE_HEIGHT + GROUP_PADDING * 2,
      });
      continue;
    }

    const g = new dagre.graphlib.Graph();
    g.setGraph({ rankdir: 'TB', ranksep: 40, nodesep: 30 });
    g.setDefaultEdgeLabel(() => ({}));

    for (const node of groupChildren) {
      g.setNode(node.id, { width: NODE_WIDTH, height: NODE_HEIGHT });
    }
    for (const edge of groupEdges) {
      g.setEdge(edge.source, edge.target);
    }

    dagre.layout(g);

    let maxX = 0;
    let maxY = 0;

    for (const node of groupChildren) {
      const pos = g.node(node.id);
      node.position = {
        x: pos.x - NODE_WIDTH / 2 + GROUP_PADDING,
        y: pos.y - NODE_HEIGHT / 2 + GROUP_PADDING + 30,
      };
      maxX = Math.max(maxX, pos.x + NODE_WIDTH / 2);
      maxY = Math.max(maxY, pos.y + NODE_HEIGHT / 2);
    }

    groupSizes.set(group.id, {
      width: maxX + GROUP_PADDING * 2,
      height: maxY + GROUP_PADDING * 2 + 30,
    });
  }

  // Pass 2: Layout groups and free nodes relative to each other
  const g2 = new dagre.graphlib.Graph();
  g2.setGraph({ rankdir: 'TB', ranksep: 60, nodesep: 60 });
  g2.setDefaultEdgeLabel(() => ({}));

  for (const group of groups) {
    const size = groupSizes.get(group.id) ?? { width: 200, height: 150 };
    g2.setNode(group.id, { width: size.width, height: size.height });
  }

  for (const node of freeNodes) {
    g2.setNode(node.id, { width: NODE_WIDTH, height: NODE_HEIGHT });
  }

  for (const edge of edges) {
    const sourceNode = nodes.find((n) => n.id === edge.source);
    const targetNode = nodes.find((n) => n.id === edge.target);

    const sourceGroup = sourceNode?.parentId ?? sourceNode?.id;
    const targetGroup = targetNode?.parentId ?? targetNode?.id;

    if (sourceGroup && targetGroup && sourceGroup !== targetGroup) {
      if (g2.hasNode(sourceGroup) && g2.hasNode(targetGroup)) {
        g2.setEdge(sourceGroup, targetGroup);
      }
    }
  }

  dagre.layout(g2);

  for (const group of groups) {
    const pos = g2.node(group.id);
    const size = groupSizes.get(group.id) ?? { width: 200, height: 150 };
    group.position = { x: pos.x - size.width / 2, y: pos.y - size.height / 2 };
    group.style = { ...group.style, width: size.width, height: size.height };
  }

  for (const node of freeNodes) {
    const pos = g2.node(node.id);
    node.position = { x: pos.x - NODE_WIDTH / 2, y: pos.y - NODE_HEIGHT / 2 };
  }

  return { nodes: [...groups, ...children, ...freeNodes], edges };
}
