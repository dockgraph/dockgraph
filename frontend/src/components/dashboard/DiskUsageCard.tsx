import { memo } from "react";
import { useTheme } from "../../theme";
import { DashboardCard } from "./DashboardCard";
import { useDiskUsage } from "../../hooks/useDiskUsage";
import { formatBytes } from "../../utils/format";
import { STATUS_COLORS } from "./palette";

const SEGMENTS = [
  { key: "images" as const, label: "Images", color: STATUS_COLORS.blue },
  { key: "containers" as const, label: "Containers", color: STATUS_COLORS.green },
  { key: "volumes" as const, label: "Volumes", color: STATUS_COLORS.amber },
  { key: "buildCache" as const, label: "Build cache", color: STATUS_COLORS.purple },
];

export const DiskUsageCard = memo(function DiskUsageCard() {
  const { theme } = useTheme();
  const { data } = useDiskUsage();

  if (!data) {
    return <DashboardCard title="Disk Usage" loading />;
  }

  const values = SEGMENTS.map(s => ({ ...s, value: data[s.key].total }));
  const total = values.reduce((sum, s) => sum + s.value, 0);
  const reclaimable = SEGMENTS.reduce((sum, s) => sum + data[s.key].reclaimable, 0);

  const totalBadge = (
    <span style={{ fontSize: 12, fontWeight: 600, color: theme.nodeText, fontFamily: "monospace", lineHeight: 1 }}>
      {formatBytes(total)}
    </span>
  );

  return (
    <DashboardCard title="Disk Usage" badge={totalBadge}>
      <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
        {/* Stacked bar */}
        <div style={{ display: "flex", height: 8, borderRadius: 4, overflow: "hidden", background: theme.panelBorder }}>
          {values.map(s => {
            const pct = total > 0 ? (s.value / total) * 100 : 0;
            if (pct < 0.5) return null;
            return <div key={s.key} style={{ width: `${pct}%`, background: s.color, transition: "width 0.3s ease" }} />;
          })}
        </div>

        {/* Legend */}
        <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
          {values.map(s => (
            <div key={s.key} style={{ display: "flex", alignItems: "center", gap: 6 }}>
              <span style={{ width: 7, height: 7, borderRadius: 2, background: s.color, flexShrink: 0 }} />
              <span style={{ fontSize: 11, color: theme.nodeSubtext, flex: 1 }}>{s.label}</span>
              <span style={{ fontSize: 11, color: theme.nodeText, fontFamily: "monospace" }}>{formatBytes(s.value)}</span>
            </div>
          ))}
        </div>

        {/* Reclaimable */}
        {reclaimable > 0 && (
          <div style={{ fontSize: 10, color: theme.nodeSubtext, borderTop: `1px solid ${theme.panelBorder}`, paddingTop: 6 }}>
            {formatBytes(reclaimable)} reclaimable
          </div>
        )}
      </div>
    </DashboardCard>
  );
});
