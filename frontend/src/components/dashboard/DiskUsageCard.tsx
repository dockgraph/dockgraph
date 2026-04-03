import { memo } from "react";
import { useTheme } from "../../theme";
import { DashboardCard } from "./DashboardCard";
import { useDiskUsage } from "../../hooks/useDiskUsage";
import { formatBytes } from "../../utils/format";

const COLORS = ["#3b82f6", "#22c55e", "#f59e0b", "#8b5cf6"];

export const DiskUsageCard = memo(function DiskUsageCard() {
  const { theme } = useTheme();
  const { data } = useDiskUsage();

  if (!data) {
    return (
      <DashboardCard title="Disk Usage">
        <span style={{ fontSize: 12, color: theme.nodeSubtext }}>Loading...</span>
      </DashboardCard>
    );
  }

  const segments = [
    { label: "Images", value: data.images.total, color: COLORS[0] },
    { label: "Containers", value: data.containers.total, color: COLORS[1] },
    { label: "Volumes", value: data.volumes.total, color: COLORS[2] },
    { label: "Build cache", value: data.buildCache.total, color: COLORS[3] },
  ];

  const total = segments.reduce((sum, s) => sum + s.value, 0);
  const reclaimable =
    data.images.reclaimable +
    data.containers.reclaimable +
    data.volumes.reclaimable +
    data.buildCache.reclaimable;

  return (
    <DashboardCard title="Disk Usage">
      <div style={{ display: "flex", height: 12, borderRadius: 4, overflow: "hidden", background: theme.panelBorder }}>
        {segments.map(s => {
          const pct = total > 0 ? (s.value / total) * 100 : 0;
          if (pct < 1) return null;
          return <div key={s.label} style={{ width: `${pct}%`, background: s.color }} />;
        })}
      </div>
      <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
        {segments.map(s => (
          <div key={s.label} style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 12 }}>
            <span style={{ width: 8, height: 8, borderRadius: 2, background: s.color, flexShrink: 0 }} />
            <span style={{ color: theme.nodeSubtext, flex: 1 }}>{s.label}</span>
            <span style={{ color: theme.nodeText, fontFamily: "monospace" }}>{formatBytes(s.value)}</span>
          </div>
        ))}
        <div style={{
          borderTop: `1px solid ${theme.panelBorder}`,
          paddingTop: 4,
          marginTop: 2,
          fontSize: 11,
          color: theme.nodeSubtext,
        }}>
          Total: {formatBytes(total)} — {formatBytes(reclaimable)} reclaimable
        </div>
      </div>
    </DashboardCard>
  );
});
