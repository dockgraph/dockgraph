import { memo } from 'react';
import type { ContainerStatsData } from '../types/stats';
import { formatBytesShort } from '../utils/formatBytes';
import { cpuColor } from '../utils/colors';
import { useTheme } from '../theme';

interface StatsMiniProps {
  stats: ContainerStatsData | undefined;
}

export const StatsMini = memo(function StatsMini({ stats }: StatsMiniProps) {
  const { theme } = useTheme();
  if (!stats) return null;

  const color = cpuColor(stats.cpuPercent, stats.cpuThrottled);
  const cpuWidth = Math.min(stats.cpuPercent, 100);
  const isThrottled = stats.cpuThrottled > 0;

  return (
    <div style={{ marginTop: 3, display: 'flex', alignItems: 'center', gap: 4 }}>
      <div
        title={`CPU: ${stats.cpuPercent.toFixed(1)}%${isThrottled ? ` (throttled ${stats.cpuThrottled.toFixed(0)}%)` : ''}`}
        style={{
          width: 40,
          height: 3,
          background: theme.portBg,
          borderRadius: 1.5,
          overflow: 'hidden',
          position: 'relative',
        }}
      >
        <div
          style={{
            width: `${cpuWidth}%`,
            height: '100%',
            background: isThrottled
              ? `repeating-linear-gradient(45deg, ${color}, ${color} 2px, transparent 2px, transparent 4px)`
              : color,
            borderRadius: 1.5,
          }}
        />
      </div>
      <span style={{ fontSize: 8, color: theme.nodeSubtext, whiteSpace: 'nowrap' }}>
        {stats.cpuPercent.toFixed(0)}%{stats.memUsage > 0 ? ` · ${formatBytesShort(stats.memUsage)}` : ''}
      </span>
    </div>
  );
});
