import { describe, it, expect } from 'vitest';
import type { ElkNode } from 'elkjs/lib/elk.bundled';
import { smoothUturns, extractEdgePaths } from './edgePaths';

describe('smoothUturns', () => {
  it('returns points unchanged when no U-turns exist', () => {
    const points = [
      { x: 0, y: 0 },
      { x: 100, y: 0 },
      { x: 100, y: 100 },
    ];
    const result = smoothUturns(points);
    expect(result).toHaveLength(3);
    expect(result[0]).toEqual({ x: 0, y: 0 });
    expect(result[2]).toEqual({ x: 100, y: 100 });
  });

  it('removes a horizontal U-turn stub', () => {
    // A→B right, B→C down (short), C→D left = horizontal U-turn
    const points = [
      { x: 0, y: 0 },
      { x: 50, y: 0 },    // right
      { x: 50, y: 10 },   // down (short stub < 30)
      { x: 0, y: 10 },    // left (reversal)
    ];
    const result = smoothUturns(points);
    expect(result.length).toBeLessThan(4);
  });

  it('removes a vertical U-turn stub', () => {
    // A→B down, B→C right (short), C→D up = vertical U-turn
    const points = [
      { x: 0, y: 0 },
      { x: 0, y: 50 },    // down
      { x: 10, y: 50 },   // right (short stub < 30)
      { x: 10, y: 0 },    // up (reversal)
    ];
    const result = smoothUturns(points);
    expect(result.length).toBeLessThan(4);
  });

  it('preserves large non-U-turn bends', () => {
    // Stub length > UTURN_THRESHOLD (30), so it should not be removed
    const points = [
      { x: 0, y: 0 },
      { x: 50, y: 0 },
      { x: 50, y: 100 },  // large vertical segment
      { x: 0, y: 100 },
    ];
    const result = smoothUturns(points);
    expect(result).toHaveLength(4);
  });

  it('handles single segment (2 points)', () => {
    const points = [{ x: 0, y: 0 }, { x: 10, y: 10 }];
    const result = smoothUturns(points);
    expect(result).toHaveLength(2);
  });

  it('handles empty input', () => {
    expect(smoothUturns([])).toEqual([]);
  });

  it('collapses horizontal U-turn where start and end align vertically', () => {
    // A and D share the same x — the two middle points should collapse completely
    const points = [
      { x: 30, y: 0 },
      { x: 50, y: 0 },   // right
      { x: 50, y: 10 },  // down (stub < 30)
      { x: 30, y: 10 },  // left back to same x
    ];
    const result = smoothUturns(points);
    // Should remove the two middle points entirely
    expect(result).toHaveLength(2);
    expect(result[0]).toEqual({ x: 30, y: 0 });
    expect(result[1]).toEqual({ x: 30, y: 10 });
  });

  it('collapses vertical U-turn where start and end align horizontally', () => {
    const points = [
      { x: 0, y: 30 },
      { x: 0, y: 50 },   // down
      { x: 10, y: 50 },  // right (stub < 30)
      { x: 10, y: 30 },  // up back to same y
    ];
    const result = smoothUturns(points);
    expect(result).toHaveLength(2);
  });

  it('does not collapse U-turn at exactly UTURN_THRESHOLD', () => {
    // Stub length = 30 = UTURN_THRESHOLD, should NOT collapse
    const points = [
      { x: 0, y: 0 },
      { x: 50, y: 0 },
      { x: 50, y: 30 },  // exactly 30
      { x: 0, y: 30 },
    ];
    const result = smoothUturns(points);
    expect(result).toHaveLength(4);
  });

  it('handles consecutive U-turns iteratively', () => {
    // Two U-turns in sequence — first pass removes one, second removes the other
    const points = [
      { x: 0, y: 0 },
      { x: 20, y: 0 },
      { x: 20, y: 5 },
      { x: 0, y: 5 },    // first U-turn
      { x: 0, y: 10 },
      { x: 20, y: 10 },
      { x: 20, y: 15 },
      { x: 0, y: 15 },   // second U-turn
    ];
    const result = smoothUturns(points);
    expect(result.length).toBeLessThan(8);
  });
});

// --- extractEdgePaths ---

describe('extractEdgePaths', () => {
  it('builds an SVG path from a single section', () => {
    const node: ElkNode = {
      id: 'root',
      edges: [{
        id: 'e1',
        sources: ['a'],
        targets: ['b'],
        sections: [{
          startPoint: { x: 0, y: 0 },
          endPoint: { x: 100, y: 100 },
          bendPoints: [{ x: 50, y: 0 }, { x: 50, y: 100 }],
        }],
      }] as any,
    };
    const out = new Map<string, string>();

    extractEdgePaths(node, 0, 0, out);

    expect(out.has('e1')).toBe(true);
    const path = out.get('e1')!;
    expect(path).toMatch(/^M 0 0/);
    expect(path).toContain('L 50 0');
    expect(path).toContain('L 50 100');
    expect(path).toContain('L 100 100');
  });

  it('applies coordinate offsets', () => {
    const node: ElkNode = {
      id: 'root',
      edges: [{
        id: 'e1',
        sources: ['a'],
        targets: ['b'],
        sections: [{
          startPoint: { x: 10, y: 20 },
          endPoint: { x: 30, y: 40 },
        }],
      }] as any,
    };
    const out = new Map<string, string>();

    extractEdgePaths(node, 100, 200, out);

    expect(out.get('e1')).toBe('M 110 220 L 130 240');
  });

  it('recurses into children with accumulated offsets', () => {
    const node: ElkNode = {
      id: 'root',
      children: [{
        id: 'child',
        x: 50,
        y: 60,
        edges: [{
          id: 'e1',
          sources: ['a'],
          targets: ['b'],
          sections: [{
            startPoint: { x: 0, y: 0 },
            endPoint: { x: 10, y: 10 },
          }],
        }] as any,
      }],
    };
    const out = new Map<string, string>();

    extractEdgePaths(node, 0, 0, out);

    expect(out.get('e1')).toBe('M 50 60 L 60 70');
  });

  it('deduplicates junction points between consecutive sections', () => {
    const node: ElkNode = {
      id: 'root',
      edges: [{
        id: 'e1',
        sources: ['a'],
        targets: ['b'],
        sections: [
          { startPoint: { x: 0, y: 0 }, endPoint: { x: 50, y: 50 } },
          { startPoint: { x: 50, y: 50 }, endPoint: { x: 100, y: 100 } },
        ],
      }] as any,
    };
    const out = new Map<string, string>();

    extractEdgePaths(node, 0, 0, out);

    // 50,50 appears once (deduped), not twice
    expect(out.get('e1')).toBe('M 0 0 L 50 50 L 100 100');
  });

  it('keeps non-duplicate junction points from separate sections', () => {
    const node: ElkNode = {
      id: 'root',
      edges: [{
        id: 'e1',
        sources: ['a'],
        targets: ['b'],
        sections: [
          { startPoint: { x: 0, y: 0 }, endPoint: { x: 50, y: 50 } },
          { startPoint: { x: 60, y: 60 }, endPoint: { x: 100, y: 100 } },
        ],
      }] as any,
    };
    const out = new Map<string, string>();

    extractEdgePaths(node, 0, 0, out);

    // Both section start/end points present
    expect(out.get('e1')).toBe('M 0 0 L 50 50 L 60 60 L 100 100');
  });

  it('skips edges with no sections', () => {
    const node: ElkNode = {
      id: 'root',
      edges: [{ id: 'e1', sources: ['a'], targets: ['b'] }] as any,
    };
    const out = new Map<string, string>();

    extractEdgePaths(node, 0, 0, out);

    expect(out.size).toBe(0);
  });

  it('skips edges with empty sections array', () => {
    const node: ElkNode = {
      id: 'root',
      edges: [{
        id: 'e1', sources: ['a'], targets: ['b'], sections: [],
      }] as any,
    };
    const out = new Map<string, string>();

    extractEdgePaths(node, 0, 0, out);

    expect(out.size).toBe(0);
  });

  it('respects parent-level precedence (does not overwrite existing ids)', () => {
    const out = new Map([['e1', 'M 0 0 L 999 999']]);
    const node: ElkNode = {
      id: 'root',
      edges: [{
        id: 'e1',
        sources: ['a'],
        targets: ['b'],
        sections: [{ startPoint: { x: 1, y: 1 }, endPoint: { x: 2, y: 2 } }],
      }] as any,
    };

    extractEdgePaths(node, 0, 0, out);

    expect(out.get('e1')).toBe('M 0 0 L 999 999');
  });

  it('handles deeply nested hierarchy', () => {
    const node: ElkNode = {
      id: 'root',
      children: [{
        id: 'level1',
        x: 10,
        y: 20,
        children: [{
          id: 'level2',
          x: 30,
          y: 40,
          edges: [{
            id: 'e1',
            sources: ['a'],
            targets: ['b'],
            sections: [{
              startPoint: { x: 0, y: 0 },
              endPoint: { x: 5, y: 5 },
            }],
          }] as any,
        }],
      }],
    };
    const out = new Map<string, string>();

    extractEdgePaths(node, 0, 0, out);

    // Offset = 10+30=40 for x, 20+40=60 for y
    expect(out.get('e1')).toBe('M 40 60 L 45 65');
  });

  it('handles node with no edges and no children', () => {
    const node: ElkNode = { id: 'root' };
    const out = new Map<string, string>();

    extractEdgePaths(node, 0, 0, out);

    expect(out.size).toBe(0);
  });
});
