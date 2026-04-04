import { memo } from "react";
import { useTheme } from "../../theme";
import { DashboardCard } from "./DashboardCard";
import { useImages } from "../../hooks/useImages";
import { formatBytes } from "../../utils/format";
import { STATUS_COLORS } from "./palette";

export const ImagesCard = memo(function ImagesCard() {
  const { theme } = useTheme();
  const { data } = useImages();

  if (!data) {
    return <DashboardCard title="Images" loading />;
  }

  const totalBadge = (
    <span style={{ fontSize: 18, fontWeight: 700, color: theme.nodeText, fontFamily: "monospace", lineHeight: 1 }}>
      {data.total}
    </span>
  );

  return (
    <DashboardCard title="Images" badge={totalBadge}>
      <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
          <span style={{ fontSize: 11, color: theme.nodeSubtext }}>Total size</span>
          <span style={{ fontSize: 11, color: theme.nodeText, fontFamily: "monospace" }}>{formatBytes(data.totalSize)}</span>
        </div>
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
          <span style={{ fontSize: 11, color: theme.nodeSubtext }}>Tags</span>
          <span style={{ fontSize: 11, color: theme.nodeText, fontFamily: "monospace" }}>{data.uniqueTags}</span>
        </div>
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
          <span style={{ fontSize: 11, color: theme.nodeSubtext }}>Dangling</span>
          <span style={{ fontSize: 11, color: data.dangling > 0 ? STATUS_COLORS.amber : theme.nodeText, fontFamily: "monospace" }}>
            {data.dangling}
          </span>
        </div>
        {data.dangling > 0 && (
          <div style={{ fontSize: 10, color: theme.nodeSubtext, borderTop: `1px solid ${theme.panelBorder}`, paddingTop: 6 }}>
            {formatBytes(data.danglingSize)} reclaimable
          </div>
        )}
      </div>
    </DashboardCard>
  );
});
