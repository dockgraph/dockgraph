import { useCallback, useEffect, useRef, useState } from 'react';
import type { LogLine } from '../types/stats';
import { parseLogEvent, logLineId } from '../utils/logParser';
import { LOG_BUFFER_SIZE } from '../utils/constants';

/** Parsed response from the history endpoint. */
interface HistoryResponse {
  lines: { stream: string; line: string; timestamp?: string; container?: string }[];
}

export interface LogsResult {
  lines: LogLine[];
  connected: boolean;
  loading: boolean;
  loadingMore: boolean;
  hasMore: boolean;
  loadMore: () => void;
  /**
   * True when the buffer no longer reaches the newest line: it was cut to
   * make room for older lines, or the live tail filled it while the reader
   * was scrolled up. Live lines wait until loadNewer catches up.
   */
  hasNewer: boolean;
  loadingNewer: boolean;
  /** Replaces the buffer with the newest page and resumes the live tail. */
  loadNewer: () => void;
  /** Tells the hook whether the reader is at the bottom, following the tail. */
  onFollowChange: (following: boolean) => void;
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
  /** Max lines kept in memory. Defaults to LOG_BUFFER_SIZE. */
  bufferSize?: number;
  /** When set, older pages are de-duplicated against the buffer by this key. */
  dedupeKey?: (line: LogLine) => string;
}

const DEFAULT_PAGE_SIZE = 200;

/** Converts a raw history entry to a LogLine with a stable ID. */
function toLogLine(entry: HistoryResponse['lines'][number]): LogLine {
  return {
    id: logLineId(),
    stream: (entry.stream === 'stderr' ? 'stderr' : 'stdout') as LogLine['stream'],
    text: entry.line,
    timestamp: entry.timestamp,
    container: entry.container,
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
 * 4. On loadNewer(): fetch the newest page again and reopen the live tail
 *
 * The buffer is always one contiguous run of lines. While the reader follows
 * the tail, live lines push the oldest out. When it can't reach the newest
 * line any more (older lines took the room, or the reader scrolled up and
 * the tail would push out what they read), it is detached: live lines are
 * held back rather than appended after a hole, until loadNewer catches up.
 */
export function useLogs({ historyUrl, streamUrl, active, pageSize = DEFAULT_PAGE_SIZE, bufferSize = LOG_BUFFER_SIZE, dedupeKey }: LogsConfig): LogsResult {
  const [lines, setLines] = useState<LogLine[]>([]);
  const [connected, setConnected] = useState(false);
  const [loading, setLoading] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [hasMore, setHasMore] = useState(true);
  const [hasNewer, setHasNewer] = useState(false);
  const [loadingNewer, setLoadingNewer] = useState(false);

  const linesRef = useRef<LogLine[]>([]);
  const sourceRef = useRef<EventSource | null>(null);
  const abortRef = useRef<AbortController | null>(null);
  const loadMoreAbortRef = useRef<AbortController | null>(null);
  const loadNewerAbortRef = useRef<AbortController | null>(null);
  // Refs, not state: read by the SSE batch flush outside React's render.
  const detachedRef = useRef(false);
  const followingRef = useRef(true);

  const detach = useCallback(() => {
    detachedRef.current = true;
    setHasNewer(true);
  }, []);

  const onFollowChange = useCallback((following: boolean) => {
    followingRef.current = following;
  }, []);

  // Tears down the EventSource without triggering a state update.
  // Use inside effects to avoid the set-state-in-effect lint rule.
  const teardownSSE = useCallback(() => {
    sourceRef.current?.close();
    sourceRef.current = null;
  }, []);

  // Fetch a page of historical log lines from the REST endpoint. A failed
  // request (the server busy, a node unreachable) is an error, not an empty
  // page: an empty page would end the history.
  const fetchHistory = useCallback(async (url: string, signal: AbortSignal): Promise<LogLine[]> => {
    const resp = await fetch(url, { signal });
    if (!resp.ok) throw new Error(`log history request failed: ${resp.status}`);
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

      // A batch from a stream that was since replaced (loadNewer) is already
      // in the page that replaced it.
      if (sourceRef.current !== source) return;
      // Detached, the buffer doesn't end at the newest line: these would
      // land after a hole. loadNewer brings them back.
      if (detachedRef.current) return;
      const buf = linesRef.current;
      // Scrolled up with a full buffer, the oldest lines may be the ones
      // being read: stop the tail rather than pull them away.
      if (!followingRef.current && buf.length + newLines.length > bufferSize) {
        detach();
        return;
      }
      buf.push(...newLines);
      if (buf.length > bufferSize) {
        buf.splice(0, buf.length - bufferSize);
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
  }, [bufferSize, detach]);

  // Shows the newest page and opens the live tail right after its newest
  // line, so no line is skipped between the two.
  const showLatest = useCallback((fetched: LogLine[]) => {
    linesRef.current = fetched;
    setLines([...fetched]);
    // A page can hold fewer lines than asked for while older ones exist (the
    // server bounds a page's size, and a TTY joins Docker's pieces of a long
    // line), so only an empty page ends the history.
    setHasMore(fetched.length > 0);
    detachedRef.current = false;
    setHasNewer(false);
    if (streamUrl) {
      const newest = fetched.length > 0 ? fetched[fetched.length - 1].timestamp : undefined;
      openSSE(newest ? appendParam(streamUrl, `since=${encodeURIComponent(newest)}`) : streamUrl);
    }
  }, [streamUrl, openSSE]);

  // Main effect: fetch initial history, then open SSE.
  useEffect(() => {
    // Teardown previous resources.
    teardownSSE();
    abortRef.current?.abort();
    loadMoreAbortRef.current?.abort();
    loadNewerAbortRef.current?.abort();
    linesRef.current = [];
    detachedRef.current = false;

    if (!historyUrl || !active) {
      // Defer state reset to avoid synchronous setState in the effect body.
      const id = requestAnimationFrame(() => {
        setConnected(false);
        setLines([]);
        setLoading(false);
        setHasMore(true);
        setHasNewer(false);
        setLoadingNewer(false);
      });
      return () => cancelAnimationFrame(id);
    }

    const abort = new AbortController();
    abortRef.current = abort;

    // Reset state and start the fetch in the same deferred callback. Starting
    // the fetch only after the reset guarantees its result is applied after the
    // reset — otherwise a fast history response (local backend answers in ~1ms,
    // well under a frame) resolves first and the reset clobbers the lines,
    // leaving the panel stuck on "Loading logs...".
    const resetId = requestAnimationFrame(() => {
      setConnected(false);
      setLines([]);
      setHasMore(true);
      setHasNewer(false);
      setLoadingNewer(false);
      setLoading(true);

      const url = appendParam(historyUrl, `limit=${pageSize}`);

      fetchHistory(url, abort.signal)
        .then((fetched) => {
          if (abort.signal.aborted) return;
          showLatest(fetched);
          setLoading(false);
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
    });

    return () => {
      cancelAnimationFrame(resetId);
      abort.abort();
      loadMoreAbortRef.current?.abort();
      loadNewerAbortRef.current?.abort();
      teardownSSE();
    };
  }, [historyUrl, streamUrl, active, pageSize, fetchHistory, openSSE, showLatest, teardownSSE]);

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

        // Lines at or after the cursor are already in the buffer. Agents of
        // earlier versions still send the line at the cursor (Docker's
        // until includes it).
        let older = fetched.filter((l) => !l.timestamp || l.timestamp < oldest);
        if (older.length === 0) {
          setHasMore(false);
          setLoadingMore(false);
          return;
        }

        setHasMore(true); // see showLatest: only an empty page ends the history

        // Drop lines already in the buffer (overlapping pages) when a key is set.
        if (dedupeKey) {
          const seen = new Set(linesRef.current.map(dedupeKey));
          older = older.filter((l) => !seen.has(dedupeKey(l)));
          if (older.length === 0) {
            setHasMore(false);
            setLoadingMore(false);
            return;
          }
        }

        // Prepend older lines. Over the limit, the newest lines give way so
        // the buffer stays one contiguous run; it no longer reaches the live
        // tail, so detach until loadNewer.
        const buf = linesRef.current;
        linesRef.current = [...older, ...buf];
        if (linesRef.current.length > bufferSize) {
          linesRef.current = linesRef.current.slice(0, bufferSize);
          detach();
        }
        setLines([...linesRef.current]);
        setLoadingMore(false);
      })
      .catch(() => {
        if (!abort.signal.aborted) {
          setLoadingMore(false);
        }
      });
  }, [historyUrl, loadingMore, hasMore, pageSize, bufferSize, dedupeKey, fetchHistory, detach]);

  // Catch up after a detach: the newest page replaces the buffer and the live
  // tail resumes after it. Older lines are one scroll-up away again.
  const loadNewer = useCallback(() => {
    if (!historyUrl || loadingNewer || !detachedRef.current) return;

    setLoadingNewer(true);
    loadNewerAbortRef.current?.abort();
    const abort = new AbortController();
    loadNewerAbortRef.current = abort;

    fetchHistory(appendParam(historyUrl, `limit=${pageSize}`), abort.signal)
      .then((fetched) => {
        if (abort.signal.aborted) return;
        teardownSSE();
        showLatest(fetched);
        setLoadingNewer(false);
      })
      .catch(() => {
        if (!abort.signal.aborted) {
          setLoadingNewer(false);
        }
      });
  }, [historyUrl, loadingNewer, pageSize, fetchHistory, showLatest, teardownSSE]);

  return { lines, connected, loading, loadingMore, hasMore, loadMore, hasNewer, loadingNewer, loadNewer, onFollowChange };
}
