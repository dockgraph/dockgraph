import { useMemo } from 'react';
import { useResourceDetail } from './useResourceDetail';
import type { VolumeDetail } from '../types/stats';

/**
 * Fetches volume inspect data when the volume name changes.
 * Cancels in-flight requests on name change or unmount.
 */
export function useVolumeDetail(volumeName: string | null) {
  const url = useMemo(
    () => volumeName ? `/api/volumes/${encodeURIComponent(volumeName)}` : null,
    [volumeName],
  );
  return useResourceDetail<VolumeDetail>(url);
}
