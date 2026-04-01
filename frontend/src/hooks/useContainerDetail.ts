import { useMemo } from 'react';
import { useResourceDetail } from './useResourceDetail';
import type { ContainerDetail } from '../types/stats';

/**
 * Fetches container inspect data when the container ID changes.
 * Cancels in-flight requests on ID change or unmount.
 */
export function useContainerDetail(containerId: string | null) {
  const url = useMemo(
    () => containerId ? `/api/containers/${encodeURIComponent(containerId)}` : null,
    [containerId],
  );
  return useResourceDetail<ContainerDetail>(url);
}
