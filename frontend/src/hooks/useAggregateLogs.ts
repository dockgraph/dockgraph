import { useLogs, type LogsResult } from './useLogs';
import type { LogLine } from '../types/stats';

/** Larger buffer than per-container logs — the aggregate stream is higher volume. */
const AGGREGATE_BUFFER_SIZE = 3000;

/** Stable content key so overlapping history pages never drop/duplicate lines. */
export function aggregateDedupeKey(l: LogLine): string {
  return `${l.container ?? ''}|${l.timestamp ?? ''}|${l.stream}|${l.text}`;
}

/**
 * Aggregate log stream across all containers. Thin wrapper over useLogs pointing
 * at the /api/logs endpoints, with a larger buffer and page de-duplication.
 */
export function useAggregateLogs(active: boolean): LogsResult {
  return useLogs({
    historyUrl: '/api/logs/history',
    streamUrl: '/api/logs',
    active,
    bufferSize: AGGREGATE_BUFFER_SIZE,
    dedupeKey: aggregateDedupeKey,
  });
}
