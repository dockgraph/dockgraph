import { describe, it, expect } from 'vitest';
import { aggregateDedupeKey } from './useAggregateLogs';
import type { LogLine } from '../types/stats';

describe('aggregateDedupeKey', () => {
  it('keys on container + timestamp + stream + text', () => {
    const a: LogLine = { id: 1, stream: 'stdout', text: 'x', timestamp: 't', container: 'web' };
    const b: LogLine = { id: 2, stream: 'stdout', text: 'x', timestamp: 't', container: 'web' };
    const c: LogLine = { id: 3, stream: 'stdout', text: 'x', timestamp: 't', container: 'db' };
    expect(aggregateDedupeKey(a)).toBe(aggregateDedupeKey(b));
    expect(aggregateDedupeKey(a)).not.toBe(aggregateDedupeKey(c));
  });
});
