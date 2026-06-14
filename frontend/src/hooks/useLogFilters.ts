import { useState, useCallback } from 'react';
import type { LogLine } from '../types/stats';

export type Chip =
  | { kind: 'include'; container: string }
  | { kind: 'exclude'; container: string }
  | { kind: 'stream'; stream: 'stdout' | 'stderr' };

export interface LogFilters {
  text: string;
  include: string | null; // single included container, or null for all
  exclude: string[]; // excluded containers
  stream: 'stdout' | 'stderr' | null;
  /** Interpret `text` as a case-insensitive regex instead of a literal substring. */
  regex: boolean;
}

export const emptyFilters: LogFilters = { text: '', include: null, exclude: [], stream: null, regex: false };

// Compile the regex once per distinct query (the same query is tested against
// every line in a filter pass), falling back to substring on an invalid pattern.
let reCacheKey: string | null = null;
let reCache: RegExp | null = null;

function textMatches(text: string, query: string, regex: boolean): boolean {
  if (!regex) return text.toLowerCase().includes(query.toLowerCase());
  if (query !== reCacheKey) {
    reCacheKey = query;
    try {
      reCache = new RegExp(query, 'i');
    } catch {
      reCache = null; // incomplete/invalid pattern -> literal fallback
    }
  }
  if (reCache) return reCache.test(text);
  return text.toLowerCase().includes(query.toLowerCase());
}

/** Pure predicate: does a line pass the active filters? */
export function matchesFilters(line: LogLine, f: LogFilters): boolean {
  if (f.include && line.container !== f.include) return false;
  if (line.container && f.exclude.includes(line.container)) return false;
  if (f.stream && line.stream !== f.stream) return false;
  if (f.text && !textMatches(line.text, f.text, f.regex)) return false;
  return true;
}

/** Apply a chip (idempotent). Include is single-valued; exclude is a set. */
export function withChip(f: LogFilters, chip: Chip): LogFilters {
  switch (chip.kind) {
    case 'include':
      return { ...f, include: chip.container };
    case 'exclude':
      return f.exclude.includes(chip.container) ? f : { ...f, exclude: [...f.exclude, chip.container] };
    case 'stream':
      return { ...f, stream: chip.stream };
  }
}

/** Remove a chip. */
export function withoutChip(f: LogFilters, chip: Chip): LogFilters {
  switch (chip.kind) {
    case 'include':
      return { ...f, include: null };
    case 'exclude':
      return { ...f, exclude: f.exclude.filter((c) => c !== chip.container) };
    case 'stream':
      return { ...f, stream: null };
  }
}

/** React state wrapper around the pure filter helpers. */
export function useLogFilters() {
  const [filters, setFilters] = useState<LogFilters>(emptyFilters);
  const setText = useCallback((text: string) => setFilters((f) => ({ ...f, text })), []);
  const addChip = useCallback((chip: Chip) => setFilters((f) => withChip(f, chip)), []);
  const removeChip = useCallback((chip: Chip) => setFilters((f) => withoutChip(f, chip)), []);
  const toggleRegex = useCallback(() => setFilters((f) => ({ ...f, regex: !f.regex })), []);
  const clear = useCallback(() => setFilters(emptyFilters), []);
  return { filters, setText, addChip, removeChip, toggleRegex, clear };
}
