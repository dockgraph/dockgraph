import type { Edge as RFEdge } from '@xyflow/react';

/**
 * Identifies connected components among top-level graph elements.
 *
 * ELK needs connected components wrapped in sub-graphs for proper layout
 * of independent clusters (e.g. separate compose stacks that share no
 * networks or volumes). Each component gets its own wrapper node so ELK
 * can position them independently.
 *
 * @param topIds - IDs of top-level elements (groups + ungrouped nodes)
 * @param edges - All graph edges
 * @param childToParent - Maps child node IDs to their parent group ID
 * @returns Arrays of connected top-level IDs
 */
export function findComponents(
  topIds: string[],
  edges: RFEdge[],
  childToParent: Map<string, string>,
): string[][] {
  const topIdSet = new Set(topIds);

  function topOf(id: string): string | undefined {
    return childToParent.get(id) ?? (topIdSet.has(id) ? id : undefined);
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
