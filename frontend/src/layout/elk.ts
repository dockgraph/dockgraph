import ELK, { type ElkNode } from 'elkjs/lib/elk.bundled';
import type { Node as RFNode, Edge as RFEdge } from '@xyflow/react';
import { findComponents } from './components';
import { extractEdgePaths } from './edgePaths';
import { classifyNodes, buildElkChildren, wrapComponents } from './elkGraph';
import { applyElkPositions } from './elkPositions';

const elk = new ELK();

const MIN_NODE_WIDTH = 140;
const MAX_NODE_WIDTH = 250;
const NODE_PADDING = 52; // icon + status dot + margins

const MAX_LABEL_CACHE_SIZE = 1024;
const labelWidthCache = new Map<string, number>();
let sharedCanvas: CanvasRenderingContext2D | null = null;

function getCanvasContext(): CanvasRenderingContext2D {
  if (!sharedCanvas) {
    sharedCanvas = document.createElement('canvas').getContext('2d')!;
  }
  return sharedCanvas;
}

/**
 * Measures the widest node label using an off-screen canvas, then clamps the
 * result so all nodes share a uniform width within [MIN_NODE_WIDTH, MAX_NODE_WIDTH].
 * Individual label widths are cached to avoid redundant measurements.
 */
function measureNodeWidth(nodes: RFNode[]): number {
  const ctx = getCanvasContext();

  let maxW = 0;
  for (const n of nodes) {
    if (n.type === 'networkGroup') continue;
    const label = (n.data as { dgNode: { name: string } }).dgNode.name;
    const cacheKey = `${n.type}:${label}`;
    let textW = labelWidthCache.get(cacheKey);
    if (textW === undefined) {
      const isVolume = n.type === 'volumeNode';
      ctx.font = isVolume ? '600 11px sans-serif' : '600 12px sans-serif';
      textW = ctx.measureText(label).width;
      if (labelWidthCache.size >= MAX_LABEL_CACHE_SIZE) {
        const oldest = labelWidthCache.keys().next().value!;
        labelWidthCache.delete(oldest);
      }
      labelWidthCache.set(cacheKey, textW);
    }
    maxW = Math.max(maxW, textW + NODE_PADDING);
  }
  return Math.max(MIN_NODE_WIDTH, Math.min(MAX_NODE_WIDTH, Math.ceil(maxW)));
}

interface LayoutResult {
  nodes: RFNode[];
  edges: RFEdge[];
}

/**
 * Computes the full graph layout using the ELK (Eclipse Layout Kernel) algorithm.
 *
 * Pipeline: measure labels → find connected components → build ELK hierarchy
 * → run layout → map positions back → extract and smooth edge paths.
 */
export async function computeLayout(
  nodes: RFNode[],
  edges: RFEdge[],
): Promise<LayoutResult> {
  const nodeWidth = measureNodeWidth(nodes);
  const { groups, children, freeNodes, childToParent } = classifyNodes(nodes);

  const topIds = [...groups.map((g) => g.id), ...freeNodes.map((n) => n.id)];
  const components = findComponents(topIds, edges, childToParent);

  const nodeMap = new Map(nodes.map((n) => [n.id, n]));

  const { elkChildren, rootEdges } = buildElkChildren(
    components, nodeMap, children, edges, childToParent, nodeWidth,
  );
  const { wrappedChildren, wrappedEdges } = wrapComponents(
    components, elkChildren, rootEdges, childToParent,
  );

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
  applyElkPositions(layout, nodeMap);

  const edgePaths = new Map<string, string>();
  extractEdgePaths(layout, 0, 0, edgePaths);

  const updatedEdges = edges.map((e) => {
    const path = edgePaths.get(e.id);
    if (path) {
      return { ...e, type: 'elk', data: { ...(e.data ?? {}), path } };
    }
    return e;
  });

  const allNodes = [...groups, ...children, ...freeNodes];
  for (const n of allNodes) {
    if (n.type !== 'networkGroup') {
      n.data = { ...n.data, nodeWidth };
    }
  }

  return { nodes: allNodes, edges: updatedEdges };
}
