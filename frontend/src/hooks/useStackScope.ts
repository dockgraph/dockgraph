import { useCallback, useState } from 'react';

/** URL query parameter carrying the selected stack. */
export const STACK_PARAM = 'stack';

function readStackParam(): string | null {
  if (typeof window === 'undefined') return null;
  return new URLSearchParams(window.location.search).get(STACK_PARAM) || null;
}

/**
 * The stack (compose project / swarm stack) the whole UI is scoped to, kept
 * in sync with `?stack=` so a scoped view can be bookmarked or shared. Uses
 * history.replaceState, so switching stacks doesn't pile up history entries.
 * null means "All".
 */
export function useStackScope(): {
  selectedStack: string | null;
  setSelectedStack: (stack: string | null) => void;
} {
  const [selectedStack, setState] = useState<string | null>(readStackParam);

  const setSelectedStack = useCallback((stack: string | null) => {
    const next = stack || null;
    setState(next);
    const url = new URL(window.location.href);
    if (next) url.searchParams.set(STACK_PARAM, next);
    else url.searchParams.delete(STACK_PARAM);
    window.history.replaceState(window.history.state, '', `${url.pathname}${url.search}${url.hash}`);
  }, []);

  return { selectedStack, setSelectedStack };
}
