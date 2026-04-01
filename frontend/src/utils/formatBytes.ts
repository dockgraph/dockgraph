const UNITS = ['B', 'KB', 'MB', 'GB', 'TB'] as const;
const SHORT_UNITS = ['B', 'K', 'M', 'G', 'T'] as const;

/** Computes the unit index and scaled value for a byte count. */
function scaleBytes(bytes: number, unitCount: number): { value: number; index: number } {
  if (bytes === 0) return { value: 0, index: 0 };
  const k = 1024;
  const index = Math.min(Math.floor(Math.log(bytes) / Math.log(k)), unitCount - 1);
  return { value: bytes / Math.pow(k, index), index };
}

/** Formats a byte count into a human-readable string (e.g., "128.0 MB"). */
export function formatBytes(bytes: number, decimals = 1): string {
  if (bytes === 0) return '0 B';
  const { value, index } = scaleBytes(bytes, UNITS.length);
  return `${value.toFixed(decimals)} ${UNITS[index]}`;
}

/** Formats bytes as a short string for compact display (e.g., "128M"). */
export function formatBytesShort(bytes: number): string {
  if (bytes === 0) return '0B';
  const { value, index } = scaleBytes(bytes, SHORT_UNITS.length);
  return `${value < 10 ? value.toFixed(1) : Math.round(value)}${SHORT_UNITS[index]}`;
}
