import { describe, it, expect } from 'vitest';
import { parseLogEvent } from './logParser';

describe('parseLogEvent container tagging', () => {
  it('populates container when present in the SSE payload', () => {
    const line = parseLogEvent(JSON.stringify({ container: 'web', stream: 'stdout', line: 'hi', timestamp: '2026-06-11T10:00:00Z' }));
    expect(line?.container).toBe('web');
    expect(line?.text).toBe('hi');
  });

  it('leaves container undefined for per-container payloads', () => {
    const line = parseLogEvent(JSON.stringify({ stream: 'stdout', line: 'hi' }));
    expect(line?.container).toBeUndefined();
  });
});
