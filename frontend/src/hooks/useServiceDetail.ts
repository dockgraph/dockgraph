import { useMemo } from 'react';
import { useResourceDetail } from './useResourceDetail';
import type { ServiceDetail } from '../types/stats';

/**
 * Fetches swarm service inspect data (with its tasks) when the service name
 * changes. Cancels in-flight requests on change or unmount.
 */
export function useServiceDetail(serviceName: string | null) {
  const url = useMemo(
    () => serviceName ? `/api/services/${encodeURIComponent(serviceName)}` : null,
    [serviceName],
  );
  return useResourceDetail<ServiceDetail>(url);
}
