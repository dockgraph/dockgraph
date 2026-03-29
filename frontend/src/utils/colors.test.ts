import { describe, it, expect } from 'vitest';
import { networkColor, hashString, STATUS_COLORS, STATUS_LABELS } from './colors';

describe('hashString', () => {
  it('returns a non-negative integer', () => {
    const result = hashString('test');
    expect(result).toBeGreaterThanOrEqual(0);
    expect(Number.isInteger(result)).toBe(true);
  });

  it('returns the same value for the same input', () => {
    expect(hashString('hello')).toBe(hashString('hello'));
  });

  it('returns different values for different inputs', () => {
    expect(hashString('abc')).not.toBe(hashString('xyz'));
  });

  it('returns 0 for empty string', () => {
    expect(hashString('')).toBe(0);
  });
});

describe('networkColor', () => {
  it('returns a hex color from the palette', () => {
    const color = networkColor('my-network');
    expect(color).toMatch(/^#[0-9a-f]{6}$/);
  });

  it('returns the same color for the same name', () => {
    expect(networkColor('backend')).toBe(networkColor('backend'));
  });

  it('is deterministic across calls', () => {
    const first = networkColor('frontend');
    const second = networkColor('frontend');
    expect(first).toBe(second);
  });
});

describe('STATUS_COLORS', () => {
  it('has entries for all expected statuses', () => {
    const statuses = ['running', 'unhealthy', 'paused', 'exited', 'dead', 'created', 'not_running'];
    for (const status of statuses) {
      expect(STATUS_COLORS[status]).toBeDefined();
      expect(STATUS_COLORS[status]).toMatch(/^#[0-9a-f]{6}$/);
    }
  });
});

describe('STATUS_LABELS', () => {
  it('has a label for every status color', () => {
    for (const status of Object.keys(STATUS_COLORS)) {
      expect(STATUS_LABELS[status]).toBeDefined();
      expect(typeof STATUS_LABELS[status]).toBe('string');
    }
  });
});
