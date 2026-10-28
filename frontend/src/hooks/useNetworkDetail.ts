import { useMemo } from 'react';
import { useResourceDetail } from './useResourceDetail';
import type { NetworkDetail } from '../types/stats';

/**
 * Fetches network inspect data when the network name changes.
 * Cancels in-flight requests on name change or unmount.
 *
 * `owner` is the node holding a node-local network: the server may be one of
 * several replicas, so the request names the node to ask.
 */
export function useNetworkDetail(networkName: string | null, owner?: string) {
  const url = useMemo(() => {
    if (!networkName) return null;
    const query = owner ? `?node=${encodeURIComponent(owner)}` : '';
    return `/api/networks/${encodeURIComponent(networkName)}${query}`;
  }, [networkName, owner]);
  return useResourceDetail<NetworkDetail>(url);
}
