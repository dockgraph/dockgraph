import { useMemo } from 'react';
import { useLogs, type LogsResult } from './useLogs';

/**
 * Container-specific log viewer. Thin wrapper around the generic useLogs hook
 * that builds the correct URLs for a single container.
 */
export function useContainerLogs(containerId: string | null, active: boolean): LogsResult {
  const historyUrl = useMemo(
    () => containerId ? `/api/containers/${encodeURIComponent(containerId)}/logs/history` : null,
    [containerId],
  );

  const streamUrl = useMemo(
    () => containerId ? `/api/containers/${encodeURIComponent(containerId)}/logs` : null,
    [containerId],
  );

  return useLogs({ historyUrl, streamUrl, active });
}
