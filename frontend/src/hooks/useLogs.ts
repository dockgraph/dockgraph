import { useCallback, useEffect, useRef, useState } from 'react';
import type { LogLine } from '../types/stats';
import { parseLogEvent, logLineId } from '../utils/logParser';
import { LOG_BUFFER_SIZE } from '../utils/constants';

/** Parsed response from the history endpoint. */
interface HistoryResponse {
  lines: { stream: string; line: string; timestamp?: string }[];
}

export interface LogsResult {
  lines: LogLine[];
  connected: boolean;
  loading: boolean;
  loadingMore: boolean;
  hasMore: boolean;
  loadMore: () => void;
}

interface LogsConfig {
  /** URL for the paginated history REST endpoint. Receives `?before=<ts>&limit=<n>` params. */
  historyUrl: string | null;
  /** URL for the SSE live stream. Receives `?since=<ts>` param. */
  streamUrl: string | null;
  /** Whether the log viewer is currently visible/active. */
  active: boolean;
  /** Number of lines to fetch per page. */
  pageSize?: number;
}

const DEFAULT_PAGE_SIZE = 200;

/** Converts a raw history entry to a LogLine with a stable ID. */
function toLogLine(entry: HistoryResponse['lines'][number]): LogLine {
  return {
    id: logLineId(),
    stream: (entry.stream === 'stderr' ? 'stderr' : 'stdout') as LogLine['stream'],
    text: entry.line,
    timestamp: entry.timestamp,
  };
}

/** Appends a query parameter to a URL, handling existing params. */
function appendParam(url: string, param: string): string {
  return `${url}${url.includes('?') ? '&' : '?'}${param}`;
}

/**
 * Generic log viewer hook with REST history pagination + SSE live tail.
 * Designed to be reusable for both per-container and global log streams.
 *
 * Flow:
 * 1. Fetch initial page via REST (newest N lines)
 * 2. Open SSE with since=<newest_timestamp> for live tail
 * 3. On loadMore(): fetch older page via REST with before=<oldest_timestamp>
 */
export function useLogs({ historyUrl, streamUrl, active, pageSize = DEFAULT_PAGE_SIZE }: LogsConfig): LogsResult {
  const [lines, setLines] = useState<LogLine[]>([]);
  const [connected, setConnected] = useState(false);
  const [loading, setLoading] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [hasMore, setHasMore] = useState(true);

  const linesRef = useRef<LogLine[]>([]);
  const sourceRef = useRef<EventSource | null>(null);
  const abortRef = useRef<AbortController | null>(null);
  const loadMoreAbortRef = useRef<AbortController | null>(null);

  // Tears down the EventSource without triggering a state update.
  // Use inside effects to avoid the set-state-in-effect lint rule.
  const teardownSSE = useCallback(() => {
    sourceRef.current?.close();
    sourceRef.current = null;
  }, []);

  // Fetch a page of historical log lines from the REST endpoint.
  const fetchHistory = useCallback(async (url: string, signal: AbortSignal): Promise<LogLine[]> => {
    const resp = await fetch(url, { signal });
    if (!resp.ok) return [];
    const data: HistoryResponse = await resp.json();
    if (!data.lines) return [];
    return data.lines.map(toLogLine);
  }, []);

  // Open SSE for live streaming, appending new lines.
  const openSSE = useCallback((url: string) => {
    const source = new EventSource(url);
    sourceRef.current = source;

    source.onopen = () => setConnected(true);

    let batch: LogLine[] = [];
    let rafId = 0;

    const flush = () => {
      rafId = 0;
      if (batch.length === 0) return;
      const newLines = batch;
      batch = [];

      const buf = linesRef.current;
      buf.push(...newLines);
      if (buf.length > LOG_BUFFER_SIZE) {
        buf.splice(0, buf.length - LOG_BUFFER_SIZE);
      }
      setLines([...buf]);
    };

    source.onmessage = (event) => {
      const logLine = parseLogEvent(event.data);
      if (!logLine) return;
      batch.push(logLine);
      // Batch rapid messages into a single render via requestAnimationFrame.
      if (!rafId) {
        rafId = requestAnimationFrame(flush);
      }
    };

    source.onerror = () => {
      setConnected(false);
    };

    return () => {
      if (rafId) cancelAnimationFrame(rafId);
      source.close();
      sourceRef.current = null;
    };
  }, []);

  // Main effect: fetch initial history, then open SSE.
  useEffect(() => {
    // Teardown previous resources.
    teardownSSE();
    abortRef.current?.abort();
    loadMoreAbortRef.current?.abort();
    linesRef.current = [];

    if (!historyUrl || !active) {
      // Defer state reset to avoid synchronous setState in the effect body.
      const id = requestAnimationFrame(() => {
        setConnected(false);
        setLines([]);
        setLoading(false);
        setHasMore(true);
      });
      return () => cancelAnimationFrame(id);
    }

    // Reset state for new fetch cycle.
    const resetId = requestAnimationFrame(() => {
      setConnected(false);
      setLines([]);
      setHasMore(true);
      setLoading(true);
    });

    const abort = new AbortController();
    abortRef.current = abort;

    const url = appendParam(historyUrl, `limit=${pageSize}`);

    fetchHistory(url, abort.signal)
      .then((fetched) => {
        if (abort.signal.aborted) return;

        linesRef.current = fetched;
        setLines([...fetched]);
        setHasMore(fetched.length >= pageSize);
        setLoading(false);

        // Open SSE for live tail, starting after the newest timestamp.
        if (streamUrl) {
          const newest = fetched.length > 0 ? fetched[fetched.length - 1].timestamp : undefined;
          const sseUrl = newest
            ? appendParam(streamUrl, `since=${encodeURIComponent(newest)}`)
            : streamUrl;
          openSSE(sseUrl);
        }
      })
      .catch((err) => {
        if (abort.signal.aborted) return;
        console.error('Failed to fetch log history:', err);
        setLoading(false);

        // Fall back to SSE-only if history fetch fails.
        if (streamUrl) {
          openSSE(streamUrl);
        }
      });

    return () => {
      cancelAnimationFrame(resetId);
      abort.abort();
      loadMoreAbortRef.current?.abort();
      teardownSSE();
    };
  }, [historyUrl, streamUrl, active, pageSize, fetchHistory, openSSE, teardownSSE]);

  // Load older logs (scroll-up pagination).
  const loadMore = useCallback(() => {
    if (!historyUrl || loadingMore || !hasMore) return;

    const oldest = linesRef.current.length > 0 ? linesRef.current[0].timestamp : undefined;
    if (!oldest) {
      setHasMore(false);
      return;
    }

    setLoadingMore(true);

    const url = appendParam(historyUrl, `limit=${pageSize}&before=${encodeURIComponent(oldest)}`);
    const abort = new AbortController();
    loadMoreAbortRef.current = abort;

    fetchHistory(url, abort.signal)
      .then((fetched) => {
        if (abort.signal.aborted) return;

        if (fetched.length === 0) {
          setHasMore(false);
          setLoadingMore(false);
          return;
        }

        setHasMore(fetched.length >= pageSize);

        // Prepend older lines.
        const buf = linesRef.current;
        linesRef.current = [...fetched, ...buf];
        if (linesRef.current.length > LOG_BUFFER_SIZE) {
          linesRef.current = linesRef.current.slice(0, LOG_BUFFER_SIZE);
        }
        setLines([...linesRef.current]);
        setLoadingMore(false);
      })
      .catch(() => {
        if (!abort.signal.aborted) {
          setLoadingMore(false);
        }
      });
  }, [historyUrl, loadingMore, hasMore, pageSize, fetchHistory]);

  return { lines, connected, loading, loadingMore, hasMore, loadMore };
}
