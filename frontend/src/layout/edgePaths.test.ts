import { describe, it, expect } from 'vitest';
import { smoothUturns } from './edgePaths';

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
});
