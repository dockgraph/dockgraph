import { useCallback, useMemo, useRef, useState } from 'react';
import type { DGNode } from '../types';
import { SEARCH_DEBOUNCE_MS } from '../utils/constants';

interface FilterState {
  statuses: Set<string>;
  types: Set<string>;
}

export interface SearchFilterResult {
  query: string;
  setQuery: (q: string) => void;
  filters: FilterState;
  toggleStatus: (s: string) => void;
  toggleType: (t: string) => void;
  clearAll: () => void;
  matchingNodeIds: Set<string> | null;
  hasActiveFilter: boolean;
  totalCount: number;
  matchCount: number;
}

export function useSearchFilter(dgNodes: DGNode[]): SearchFilterResult {
  const [query, setQueryRaw] = useState('');
  const [debouncedQuery, setDebouncedQuery] = useState('');
  const [filters, setFilters] = useState<FilterState>({ statuses: new Set<string>(), types: new Set<string>() });
  const timerRef = useRef<ReturnType<typeof setTimeout>>(undefined);

  const setQuery = useCallback((q: string) => {
    setQueryRaw(q);
    clearTimeout(timerRef.current);
    timerRef.current = setTimeout(() => setDebouncedQuery(q), SEARCH_DEBOUNCE_MS);
  }, []);

  const toggleStatus = useCallback((s: string) => {
    setFilters((prev) => {
      const next = new Set<string>(prev.statuses);
      if (next.has(s)) { next.delete(s); } else { next.add(s); }
      return { ...prev, statuses: next };
    });
  }, []);

  const toggleType = useCallback((t: string) => {
    setFilters((prev) => {
      const next = new Set<string>(prev.types);
      if (next.has(t)) { next.delete(t); } else { next.add(t); }
      return { ...prev, types: next };
    });
  }, []);

  const clearAll = useCallback(() => {
    setQueryRaw('');
    setDebouncedQuery('');
    setFilters({ statuses: new Set<string>(), types: new Set<string>() });
  }, []);

  const hasActiveFilter = debouncedQuery.length > 0 || filters.statuses.size > 0 || filters.types.size > 0;

  const { matchingNodeIds, matchCount } = useMemo(() => {
    if (!hasActiveFilter) return { matchingNodeIds: null, matchCount: dgNodes.length };
    const lowerQ = debouncedQuery.toLowerCase();
    const ids = new Set<string>();
    for (const n of dgNodes) {
      if (filters.types.size > 0 && !filters.types.has(n.type)) continue;
      if (filters.statuses.size > 0 && !filters.statuses.has(n.status ?? '')) continue;
      if (lowerQ) {
        const haystack = `${n.name} ${n.image ?? ''} ${Object.values(n.labels ?? {}).join(' ')}`.toLowerCase();
        if (!haystack.includes(lowerQ)) continue;
      }
      ids.add(n.id);
    }
    return { matchingNodeIds: ids, matchCount: ids.size };
  }, [dgNodes, debouncedQuery, filters, hasActiveFilter]);

  return { query, setQuery, filters, toggleStatus, toggleType, clearAll, matchingNodeIds, hasActiveFilter, totalCount: dgNodes.length, matchCount };
}
