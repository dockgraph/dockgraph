import type { ElkNode, ElkExtendedEdge } from 'elkjs/lib/elk.bundled';

interface Point {
  x: number;
  y: number;
}

/** Minimum stub length to qualify as a U-turn artifact */
const UTURN_THRESHOLD = 30;

/**
 * Removes small U-turn artifacts from orthogonal edge paths.
 *
 * ELK sometimes routes edges with short back-and-forth stubs when navigating
 * around nodes. These look like tiny zigzags that don't add clarity. This
 * function detects 4-point patterns where the path reverses direction over
 * a short distance and collapses them into a straight segment.
 */
export function smoothUturns(points: Point[]): Point[] {
  const result = points.map((p) => ({ ...p }));
  let changed = true;

  while (changed) {
    changed = false;
    for (let i = 0; i < result.length - 3; i++) {
      const a = result[i];
      const b = result[i + 1];
      const c = result[i + 2];
      const d = result[i + 3];

      // Horizontal U-turn: A→B horizontal, B→C vertical, C→D horizontal (reversed)
      if (
        Math.abs(a.y - b.y) < 0.5 &&
        Math.abs(b.x - c.x) < 0.5 &&
        Math.abs(c.y - d.y) < 0.5
      ) {
        const abDir = Math.sign(b.x - a.x);
        const cdDir = Math.sign(d.x - c.x);
        const stubLen = Math.abs(b.y - c.y);
        if (abDir !== 0 && cdDir !== 0 && abDir !== cdDir && stubLen < UTURN_THRESHOLD) {
          if (Math.abs(a.x - d.x) < 0.5) {
            result.splice(i + 1, 2);
          } else {
            result.splice(i + 1, 2, { x: a.x, y: c.y });
          }
          changed = true;
          break;
        }
      }

      // Vertical U-turn: A→B vertical, B→C horizontal, C→D vertical (reversed)
      if (
        Math.abs(a.x - b.x) < 0.5 &&
        Math.abs(b.y - c.y) < 0.5 &&
        Math.abs(c.x - d.x) < 0.5
      ) {
        const abDir = Math.sign(b.y - a.y);
        const cdDir = Math.sign(d.y - c.y);
        const stubLen = Math.abs(b.x - c.x);
        if (abDir !== 0 && cdDir !== 0 && abDir !== cdDir && stubLen < UTURN_THRESHOLD) {
          if (Math.abs(a.y - d.y) < 0.5) {
            result.splice(i + 1, 2);
          } else {
            result.splice(i + 1, 2, { x: c.x, y: a.y });
          }
          changed = true;
          break;
        }
      }
    }
  }

  return result;
}

/**
 * Recursively extracts SVG path strings from ELK's layout result.
 *
 * ELK produces edge geometry as "sections" attached to edges at various
 * hierarchy levels. Cross-hierarchy edges may have multiple sections that
 * need concatenation. This function walks the entire node tree, accumulating
 * coordinate offsets to produce absolute paths.
 *
 * Higher-level paths take precedence — if a path was already extracted at
 * a parent level, the child-level partial path is skipped.
 */
export function extractEdgePaths(
  node: ElkNode,
  offsetX: number,
  offsetY: number,
  out: Map<string, string>,
): void {
  for (const edge of (node as { edges?: ElkExtendedEdge[] }).edges ?? []) {
    if (out.has(edge.id)) continue;

    const sections = edge.sections;
    if (!sections || sections.length === 0) continue;

    // Concatenate all sections: ELK splits cross-hierarchy edges into
    // multiple sections (one per hierarchy level crossed).
    const points: Point[] = [];
    for (const section of sections) {
      const sp: Point = {
        x: section.startPoint.x + offsetX,
        y: section.startPoint.y + offsetY,
      };
      // Deduplicate junction points between consecutive sections
      const prev = points[points.length - 1];
      if (!prev || Math.abs(sp.x - prev.x) > 0.5 || Math.abs(sp.y - prev.y) > 0.5) {
        points.push(sp);
      }
      for (const bp of section.bendPoints ?? []) {
        points.push({ x: bp.x + offsetX, y: bp.y + offsetY });
      }
      points.push({
        x: section.endPoint.x + offsetX,
        y: section.endPoint.y + offsetY,
      });
    }

    const smoothed = smoothUturns(points);
    let d = `M ${smoothed[0].x} ${smoothed[0].y}`;
    for (let i = 1; i < smoothed.length; i++) {
      d += ` L ${smoothed[i].x} ${smoothed[i].y}`;
    }
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
