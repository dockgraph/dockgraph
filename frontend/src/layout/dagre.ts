import dagre from '@dagrejs/dagre';
import type { Node as RFNode, Edge as RFEdge } from '@xyflow/react';

const NODE_WIDTH = 230;
const NODE_HEIGHT = 70;
const GROUP_PADDING_X = 30;
const GROUP_PADDING_TOP = 40; // extra space for group label
const GROUP_PADDING_BOTTOM = 20;
const NODE_GAP_X = 30;
const NODE_GAP_Y = 30;
const MAX_COLS = 3; // max columns within a group before wrapping

interface LayoutResult {
  nodes: RFNode[];
  edges: RFEdge[];
}

// Layout children within a group using dagre if they have edges,
// or a grid if they're mostly unconnected.
function layoutGroupChildren(
  groupChildren: RFNode[],
  groupEdges: RFEdge[],
): { width: number; height: number } {
  if (groupChildren.length === 0) {
    return {
      width: NODE_WIDTH + GROUP_PADDING_X * 2,
      height: NODE_HEIGHT + GROUP_PADDING_TOP + GROUP_PADDING_BOTTOM,
    };
  }

  const hasInternalEdges = groupEdges.length > 0;

  if (hasInternalEdges) {
    // Use dagre for connected graphs
    const g = new dagre.graphlib.Graph();
    g.setGraph({ rankdir: 'TB', ranksep: NODE_GAP_Y, nodesep: NODE_GAP_X });
    g.setDefaultEdgeLabel(() => ({}));

    for (const node of groupChildren) {
      g.setNode(node.id, { width: NODE_WIDTH, height: NODE_HEIGHT });
    }
    for (const edge of groupEdges) {
      g.setEdge(edge.source, edge.target);
    }

    dagre.layout(g);

    let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;

    for (const node of groupChildren) {
      const pos = g.node(node.id);
      const x = pos.x - NODE_WIDTH / 2;
      const y = pos.y - NODE_HEIGHT / 2;
      minX = Math.min(minX, x);
      minY = Math.min(minY, y);
      maxX = Math.max(maxX, x + NODE_WIDTH);
      maxY = Math.max(maxY, y + NODE_HEIGHT);
    }

    // Shift all positions so they start from padding offset
    for (const node of groupChildren) {
      const pos = g.node(node.id);
      node.position = {
        x: pos.x - NODE_WIDTH / 2 - minX + GROUP_PADDING_X,
        y: pos.y - NODE_HEIGHT / 2 - minY + GROUP_PADDING_TOP,
      };
    }

    return {
      width: (maxX - minX) + GROUP_PADDING_X * 2,
      height: (maxY - minY) + GROUP_PADDING_TOP + GROUP_PADDING_BOTTOM,
    };
  }

  // Grid layout for unconnected nodes
  const cols = Math.min(groupChildren.length, MAX_COLS);
  const rows = Math.ceil(groupChildren.length / cols);

  for (let i = 0; i < groupChildren.length; i++) {
    const col = i % cols;
    const row = Math.floor(i / cols);
    groupChildren[i].position = {
      x: GROUP_PADDING_X + col * (NODE_WIDTH + NODE_GAP_X),
      y: GROUP_PADDING_TOP + row * (NODE_HEIGHT + NODE_GAP_Y),
    };
  }

  return {
    width: cols * (NODE_WIDTH + NODE_GAP_X) - NODE_GAP_X + GROUP_PADDING_X * 2,
    height: rows * (NODE_HEIGHT + NODE_GAP_Y) - NODE_GAP_Y + GROUP_PADDING_TOP + GROUP_PADDING_BOTTOM,
  };
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

    const size = layoutGroupChildren(groupChildren, groupEdges);
    groupSizes.set(group.id, size);
  }

  // Pass 2: Layout groups and free nodes using dagre
  const g2 = new dagre.graphlib.Graph();
  g2.setGraph({ rankdir: 'LR', ranksep: 100, nodesep: 60 });
  g2.setDefaultEdgeLabel(() => ({}));

  for (const group of groups) {
    const size = groupSizes.get(group.id)!;
    g2.setNode(group.id, { width: size.width, height: size.height });
  }

  for (const node of freeNodes) {
    g2.setNode(node.id, { width: NODE_WIDTH, height: NODE_HEIGHT });
  }

  // Cross-group edges
  for (const edge of edges) {
    const sourceNode = nodes.find((n) => n.id === edge.source);
    const targetNode = nodes.find((n) => n.id === edge.target);

    const sourceGroup = sourceNode?.parentId ?? sourceNode?.id;
    const targetGroup = targetNode?.parentId ?? targetNode?.id;

    if (sourceGroup && targetGroup && sourceGroup !== targetGroup) {
      if (g2.hasNode(sourceGroup) && g2.hasNode(targetGroup)) {
        if (!g2.hasEdge(sourceGroup, targetGroup)) {
          g2.setEdge(sourceGroup, targetGroup);
        }
      }
    }
  }

  dagre.layout(g2);

  for (const group of groups) {
    const pos = g2.node(group.id);
    const size = groupSizes.get(group.id)!;
    group.position = { x: pos.x - size.width / 2, y: pos.y - size.height / 2 };
    group.style = { ...group.style, width: size.width, height: size.height };
  }

  for (const node of freeNodes) {
    const pos = g2.node(node.id);
    node.position = { x: pos.x - NODE_WIDTH / 2, y: pos.y - NODE_HEIGHT / 2 };
  }

  return { nodes: [...groups, ...children, ...freeNodes], edges };
}
