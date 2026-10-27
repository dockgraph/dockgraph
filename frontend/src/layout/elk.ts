import type { ElkNode } from 'elkjs/lib/elk.bundled';
import type { Node as RFNode, Edge as RFEdge } from '@xyflow/react';
import { findComponents } from './components';
import { extractEdgePaths } from './edgePaths';
import { classifyNodes, buildElkChildren, wrapComponents, isGroupType } from './elkGraph';
import { applyElkPositions } from './elkPositions';
import { MONO_FONT_VAR, NODE_NAME_FONT, VOLUME_NAME_FONT, REPLICA_BADGE_FONT } from '../utils/nodeStyles';
import { replicaText } from '../utils/stack';
import type { DGNode } from '../types';

// ELK's runtime (~1.6 MB) is the single largest dependency, but it only runs
// once graph data has arrived. Load it lazily on first layout so it lands in
// its own chunk instead of the initial bundle.
type ElkInstance = InstanceType<typeof import('elkjs/lib/elk.bundled').default>;
let elkPromise: Promise<ElkInstance> | null = null;
function getElk(): Promise<ElkInstance> {
  if (!elkPromise) {
    elkPromise = import('elkjs/lib/elk.bundled').then((m) => new m.default());
  }
  return elkPromise;
}

const MIN_NODE_WIDTH = 140;
const MAX_NODE_WIDTH = 280;

/**
 * Width a node spends beside its name: borders, padding and the other items
 * on the name row. Keep in step with ContainerNode, ServiceNode and VolumeNode.
 */
const NAME_ROW_CHROME: Record<string, number> = {
  containerNode: 54, // 4 border + 20 padding, and the 30px the name leaves for ☰ and the status dot
  serviceNode: 48, // 4 border + 20 padding, 6 gap, 14 ☰, 4 gap; the replica badge comes on top
  volumeNode: 58, // 4 border + 20 padding, 13 icon, 7 gap, 14 ☰
};
/** Padding and border around the replica badge text. */
const BADGE_CHROME = 12;
/** Replica counts change without a relayout, so leave room for two-digit ones. */
const BADGE_MIN_TEXT = '00/00';

const MAX_LABEL_CACHE_SIZE = 1024;
const labelWidthCache = new Map<string, number>();
let sharedCanvas: CanvasRenderingContext2D | null = null;

function getCanvasContext(): CanvasRenderingContext2D {
  if (!sharedCanvas) {
    sharedCanvas = document.createElement('canvas').getContext('2d')!;
  }
  return sharedCanvas;
}

/** Canvas font for a label style. Canvas can't resolve CSS variables, so the font stack is read from the token. */
function canvasFont(font: typeof NODE_NAME_FONT): string {
  const family = getComputedStyle(document.documentElement).getPropertyValue(MONO_FONT_VAR).trim() || 'monospace';
  return `${font.fontWeight} ${font.fontSize}px ${family}`;
}

/**
 * Waits for the label font. Names measured before it loads would be sized
 * (and cached) in a fallback font. All label sizes and weights share one
 * variable font file, so loading one size loads them all.
 */
async function labelFontReady(): Promise<void> {
  try {
    await document.fonts?.load(canvasFont(NODE_NAME_FONT));
  } catch {
    // Measure with whatever font is available.
  }
}

/** Width of `text` in `font`, measured on an off-screen canvas and cached. */
function textWidth(font: string, text: string): number {
  const cacheKey = `${font}|${text}`;
  let width = labelWidthCache.get(cacheKey);
  if (width === undefined) {
    const ctx = getCanvasContext();
    ctx.font = font;
    width = ctx.measureText(text).width;
    if (labelWidthCache.size >= MAX_LABEL_CACHE_SIZE) {
      const oldest = labelWidthCache.keys().next().value!;
      labelWidthCache.delete(oldest);
    }
    labelWidthCache.set(cacheKey, width);
  }
  return width;
}

/**
 * Finds the width the widest node needs to show its whole name, then clamps
 * it so all nodes share a uniform width within [MIN_NODE_WIDTH, MAX_NODE_WIDTH].
 * Longer names are truncated, with the full name in their tooltip.
 */
function measureNodeWidth(nodes: RFNode[]): number {
  const nameFont = canvasFont(NODE_NAME_FONT);
  const volumeFont = canvasFont(VOLUME_NAME_FONT);
  const badgeFont = canvasFont(REPLICA_BADGE_FONT);

  let maxW = 0;
  for (const n of nodes) {
    if (isGroupType(n.type)) continue;
    const dgNode = (n.data as { dgNode: DGNode }).dgNode;
    const isVolume = n.type === 'volumeNode';
    let width = textWidth(isVolume ? volumeFont : nameFont, dgNode.name)
      + (NAME_ROW_CHROME[n.type ?? ''] ?? NAME_ROW_CHROME.containerNode);
    const replicas = n.type === 'serviceNode' ? dgNode.service?.replicas : undefined;
    if (replicas) {
      const badgeText = replicaText(replicas, dgNode.status);
      width += Math.max(textWidth(badgeFont, badgeText), textWidth(badgeFont, BADGE_MIN_TEXT)) + BADGE_CHROME;
    }
    maxW = Math.max(maxW, width);
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
  await labelFontReady();
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

  const elk = await getElk();
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

  // Nodes are freshly created by classifyNodes — mutation is safe here.
  const allNodes = [...groups, ...children, ...freeNodes];
  for (const n of allNodes) {
    if (!isGroupType(n.type)) {
      n.data = { ...n.data, nodeWidth };
    }
  }

  return { nodes: allNodes, edges: updatedEdges };
}
