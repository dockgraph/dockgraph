import { describe, it, expect } from 'vitest';
import { emptyFilters, matchesFilters, withChip, withoutChip, type LogFilters } from './useLogFilters';
import type { LogLine } from '../types/stats';

const ln = (container: string, stream: 'stdout' | 'stderr', text: string): LogLine =>
  ({ id: Math.random(), container, stream, text, timestamp: 't' });

describe('matchesFilters', () => {
  it('text filter is case-insensitive substring', () => {
    const f: LogFilters = { ...emptyFilters, text: 'ERR' };
    expect(matchesFilters(ln('web', 'stdout', 'an error here'), f)).toBe(true);
    expect(matchesFilters(ln('web', 'stdout', 'ok'), f)).toBe(false);
  });

  it('include-container shows only that container', () => {
    const f = withChip(emptyFilters, { kind: 'include', container: 'web' });
    expect(matchesFilters(ln('web', 'stdout', 'x'), f)).toBe(true);
    expect(matchesFilters(ln('db', 'stdout', 'x'), f)).toBe(false);
  });

  it('exclude-container hides that container', () => {
    const f = withChip(emptyFilters, { kind: 'exclude', container: 'db' });
    expect(matchesFilters(ln('db', 'stdout', 'x'), f)).toBe(false);
    expect(matchesFilters(ln('web', 'stdout', 'x'), f)).toBe(true);
  });

  it('stream filter restricts to one stream', () => {
    const f = withChip(emptyFilters, { kind: 'stream', stream: 'stderr' });
    expect(matchesFilters(ln('web', 'stderr', 'x'), f)).toBe(true);
    expect(matchesFilters(ln('web', 'stdout', 'x'), f)).toBe(false);
  });

  it('withoutChip removes an exclusion', () => {
    let f = withChip(emptyFilters, { kind: 'exclude', container: 'db' });
    f = withoutChip(f, { kind: 'exclude', container: 'db' });
    expect(matchesFilters(ln('db', 'stdout', 'x'), f)).toBe(true);
  });

  it('regex mode interprets the query as a pattern; literal mode does not', () => {
    const asRegex: LogFilters = { ...emptyFilters, text: 'err.r', regex: true };
    expect(matchesFilters(ln('web', 'stdout', 'error'), asRegex)).toBe(true);
    const asLiteral: LogFilters = { ...emptyFilters, text: 'err.r', regex: false };
    expect(matchesFilters(ln('web', 'stdout', 'error'), asLiteral)).toBe(false);
  });

  it('regex is case-insensitive', () => {
    const f: LogFilters = { ...emptyFilters, text: '^ERR', regex: true };
    expect(matchesFilters(ln('web', 'stdout', 'error happened'), f)).toBe(true);
  });

  it('invalid regex falls back to substring (no crash while typing)', () => {
    const f: LogFilters = { ...emptyFilters, text: '[unclosed', regex: true };
    expect(matchesFilters(ln('web', 'stdout', 'has [unclosed bracket'), f)).toBe(true);
    expect(matchesFilters(ln('web', 'stdout', 'no match'), f)).toBe(false);
  });
});
