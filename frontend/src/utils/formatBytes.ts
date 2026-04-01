const UNITS = ['B', 'KB', 'MB', 'GB', 'TB'] as const;

/** Formats a byte count into a human-readable string (e.g., "128.0 MB"). */
export function formatBytes(bytes: number, decimals = 1): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(k)), UNITS.length - 1);
  const value = bytes / Math.pow(k, i);
  return `${value.toFixed(decimals)} ${UNITS[i]}`;
}

/** Formats bytes as a short string for compact display (e.g., "128M"). */
export function formatBytesShort(bytes: number): string {
  if (bytes === 0) return '0B';
  const k = 1024;
  const units = ['B', 'K', 'M', 'G', 'T'];
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(k)), units.length - 1);
  const value = bytes / Math.pow(k, i);
  return `${value < 10 ? value.toFixed(1) : Math.round(value)}${units[i]}`;
}
