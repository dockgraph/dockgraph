import { useCallback, useState } from 'react';

/** URL query parameter carrying the graph grouping mode. */
export const GROUP_PARAM = 'group';

/** How the graph view groups workloads: by network (default) or by swarm node. */
export type GroupBy = 'network' | 'node';

function readGroupParam(): GroupBy {
  if (typeof window === 'undefined') return 'network';
  return new URLSearchParams(window.location.search).get(GROUP_PARAM) === 'node' ? 'node' : 'network';
}

/**
 * The graph grouping mode, kept in sync with `?group=node` so the per-node
 * view can be bookmarked or shared. The default (network) leaves the param
 * off the URL. Uses history.replaceState, like useStackScope.
 */
export function useGroupBy(): {
  groupBy: GroupBy;
  setGroupBy: (mode: GroupBy) => void;
} {
  const [groupBy, setState] = useState<GroupBy>(readGroupParam);

  const setGroupBy = useCallback((mode: GroupBy) => {
    setState(mode);
    const url = new URL(window.location.href);
    if (mode === 'node') url.searchParams.set(GROUP_PARAM, 'node');
    else url.searchParams.delete(GROUP_PARAM);
    window.history.replaceState(window.history.state, '', `${url.pathname}${url.search}${url.hash}`);
  }, []);

  return { groupBy, setGroupBy };
}
