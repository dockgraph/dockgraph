import type { LogLine } from '../types/stats';

let nextLogId = 0;

/** Returns a monotonically increasing ID for log line keys. */
export function logLineId(): number {
  return nextLogId++;
}

/** Parses an SSE data payload into a LogLine. */
export function parseLogEvent(data: string): LogLine | null {
  try {
    const parsed = JSON.parse(data) as { stream?: string; line?: string; timestamp?: string };
    if (!parsed.line && !parsed.timestamp) return null;
    const stream = parsed.stream === 'stderr' ? 'stderr' : 'stdout';
    return { id: logLineId(), stream, text: parsed.line ?? '', timestamp: parsed.timestamp };
  } catch {
    return null;
  }
}

/** Formats a Docker timestamp to HH:MM:SS.mmm for display. */
export function formatLogTimestamp(ts: string): string {
  const d = new Date(ts);
  if (isNaN(d.getTime())) return ts.slice(11, 23);
  const hh = String(d.getHours()).padStart(2, '0');
  const mm = String(d.getMinutes()).padStart(2, '0');
  const ss = String(d.getSeconds()).padStart(2, '0');
  const ms = String(d.getMilliseconds()).padStart(3, '0');
  return `${hh}:${mm}:${ss}.${ms}`;
}
