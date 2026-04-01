import { useMemo } from 'react';
import { useResourceDetail } from './useResourceDetail';
import type { NetworkDetail } from '../types/stats';

/**
 * Fetches network inspect data when the network name changes.
 * Cancels in-flight requests on name change or unmount.
 */
export function useNetworkDetail(networkName: string | null) {
  const url = useMemo(
    () => networkName ? `/api/networks/${encodeURIComponent(networkName)}` : null,
    [networkName],
  );
  return useResourceDetail<NetworkDetail>(url);
}
