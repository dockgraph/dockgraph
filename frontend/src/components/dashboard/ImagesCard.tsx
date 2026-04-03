import { memo } from "react";
import { useTheme } from "../../theme";
import { DashboardCard } from "./DashboardCard";
import { useImages } from "../../hooks/useImages";
import { formatBytes } from "../../utils/format";

export const ImagesCard = memo(function ImagesCard() {
  const { theme } = useTheme();
  const { data } = useImages();

  if (!data) {
    return (
      <DashboardCard title="Images">
        <span style={{ fontSize: 12, color: theme.nodeSubtext }}>Loading...</span>
      </DashboardCard>
    );
  }

  const rows = [
    ["Total", String(data.total)],
    ["Size", formatBytes(data.totalSize)],
    ["Tags", String(data.uniqueTags)],
    ["Dangling", `${data.dangling} (${formatBytes(data.danglingSize)})`],
  ];

  return (
    <DashboardCard title="Images">
      <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
        {rows.map(([label, value]) => (
          <div key={label} style={{ display: "flex", justifyContent: "space-between", fontSize: 12 }}>
            <span style={{ color: theme.nodeSubtext }}>{label}</span>
            <span style={{ color: theme.nodeText, fontFamily: "monospace" }}>{value}</span>
          </div>
        ))}
      </div>
    </DashboardCard>
  );
});
