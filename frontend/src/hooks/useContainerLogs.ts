import { useMemo } from 'react';
import { useLogs, type LogsResult } from './useLogs';

/** Which log endpoint family to read: a container, or a swarm service (all its tasks). */
export type LogResource = 'container' | 'service';

/**
 * Container- (or service-) specific log viewer. Thin wrapper around the generic
 * useLogs hook that builds the correct URLs for a single resource.
 */
export function useContainerLogs(containerId: string | null, active: boolean, resource: LogResource = 'container'): LogsResult {
  const base = resource === 'service' ? '/api/services' : '/api/containers';
  const historyUrl = useMemo(
    () => containerId ? `${base}/${encodeURIComponent(containerId)}/logs/history` : null,
    [base, containerId],
  );

  const streamUrl = useMemo(
    () => containerId ? `${base}/${encodeURIComponent(containerId)}/logs` : null,
    [base, containerId],
  );

  return useLogs({ historyUrl, streamUrl, active });
}
