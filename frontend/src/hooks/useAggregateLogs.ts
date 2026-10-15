import { useMemo } from 'react';
import { useLogs, type LogsResult } from './useLogs';
import type { LogLine } from '../types/stats';

/** Larger buffer than per-container logs — the aggregate stream is higher volume. */
const AGGREGATE_BUFFER_SIZE = 3000;

/** Stable content key so overlapping history pages never drop/duplicate lines. */
export function aggregateDedupeKey(l: LogLine): string {
  return `${l.container ?? ''}|${l.timestamp ?? ''}|${l.stream}|${l.text}`;
}

/**
 * Aggregate log stream across all containers (and, in swarm mode, services).
 * Thin wrapper over useLogs pointing at the /api/logs endpoints, with a larger
 * buffer and page de-duplication. `stack` narrows it server-side to one
 * compose project / swarm stack via `?stack=`.
 */
export function useAggregateLogs(active: boolean, stack?: string | null): LogsResult {
  const query = stack ? `?stack=${encodeURIComponent(stack)}` : '';
  const historyUrl = useMemo(() => `/api/logs/history${query}`, [query]);
  const streamUrl = useMemo(() => `/api/logs${query}`, [query]);
  return useLogs({
    historyUrl,
    streamUrl,
    active,
    bufferSize: AGGREGATE_BUFFER_SIZE,
    dedupeKey: aggregateDedupeKey,
  });
}
