import { useCallback, useEffect, useRef, useState } from 'react';
import type { LogLine } from '../types/stats';
import { parseLogEvent } from '../utils/logParser';
import { LOG_BUFFER_SIZE, LOG_TAIL_DEFAULT } from '../utils/constants';

interface LogsResult {
  lines: LogLine[];
  connected: boolean;
}

/**
 * Opens an SSE connection to stream container logs.
 * Maintains a circular buffer of the most recent lines.
 */
export function useContainerLogs(containerId: string | null, active: boolean): LogsResult {
  const [lines, setLines] = useState<LogLine[]>([]);
  const [connected, setConnected] = useState(false);
  const bufferRef = useRef<LogLine[]>([]);
  const sourceRef = useRef<EventSource | null>(null);

  const close = useCallback(() => {
    sourceRef.current?.close();
    sourceRef.current = null;
    setConnected(false);
  }, []);

  useEffect(() => {
    if (!containerId || !active) {
      close();
      bufferRef.current = [];
      setLines([]);
      return;
    }

    close();
    bufferRef.current = [];
    setLines([]);

    const url = `/api/containers/${encodeURIComponent(containerId)}/logs?tail=${LOG_TAIL_DEFAULT}`;
    const source = new EventSource(url);
    sourceRef.current = source;

    source.onopen = () => setConnected(true);

    source.onmessage = (event) => {
      const logLine = parseLogEvent(event.data);
      if (!logLine) return;
      const buf = bufferRef.current;
      buf.push(logLine);
      if (buf.length > LOG_BUFFER_SIZE) {
        buf.splice(0, buf.length - LOG_BUFFER_SIZE);
      }
      setLines([...buf]);
    };

    source.onerror = () => {
      setConnected(false);
    };

    return () => {
      source.close();
      sourceRef.current = null;
    };
  }, [containerId, active, close]);

  return { lines, connected };
}
