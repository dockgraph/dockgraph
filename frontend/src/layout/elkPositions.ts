import type { ElkNode } from 'elkjs/lib/elk.bundled';
import type { Node as RFNode } from '@xyflow/react';

/**
 * Maps ELK-computed positions back to React Flow nodes. Handles both
 * component-wrapper nodes (prefixed with __comp_) and standalone top-level nodes.
 */
export function applyElkPositions(
  layout: ElkNode,
  nodeMap: Map<string, RFNode>,
): void {
  for (const topNode of layout.children ?? []) {
    const isWrapper = topNode.id.startsWith('__comp_');
    const offsetX = isWrapper ? (topNode.x ?? 0) : 0;
    const offsetY = isWrapper ? (topNode.y ?? 0) : 0;

    const elkNodes = isWrapper ? (topNode.children ?? []) : [topNode];

    for (const elkNode of elkNodes) {
      const rfNode = nodeMap.get(elkNode.id);
      if (!rfNode) continue;

      // Positions are assigned during the layout pass. Nodes are consumed immediately after.
      rfNode.position = {
        x: (elkNode.x ?? 0) + offsetX,
        y: (elkNode.y ?? 0) + offsetY,
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
  }
}
