import type { LogLine } from '../types/stats';

/** Parses an SSE data payload into a LogLine. */
export function parseLogEvent(data: string): LogLine | null {
  try {
    const parsed = JSON.parse(data) as { stream?: string; line?: string };
    if (!parsed.line) return null;
    const stream = parsed.stream === 'stderr' ? 'stderr' : 'stdout';
    const { timestamp, message } = splitTimestamp(parsed.line);
    return { stream, text: message, timestamp };
  } catch {
    return null;
  }
}

/** Splits a Docker log line into timestamp and message parts. */
function splitTimestamp(line: string): { timestamp?: string; message: string } {
  const match = line.match(/^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d+Z?)\s(.*)/);
  if (!match) return { message: line };
  return { timestamp: match[1], message: match[2] };
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
