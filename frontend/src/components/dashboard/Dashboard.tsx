import { useState, useEffect, memo } from "react";
import { useTheme } from "../../theme";
import { TimeRangeSelector } from "./TimeRangeSelector";
import { StatusSummaryCard } from "./StatusSummaryCard";
import { HostInfoCard } from "./HostInfoCard";
import { DiskUsageCard } from "./DiskUsageCard";
import { ImagesCard } from "./ImagesCard";
import { ResourceChart } from "./ResourceChart";
import { TopConsumersCard } from "./TopConsumersCard";
import { AlertsCard } from "./AlertsCard";
import { ComposeProjectsCard } from "./ComposeProjectsCard";
import { EventTimelineCard } from "./EventTimelineCard";
import { useStatsHistory, type TimeRange } from "../../hooks/useStatsHistory";
import type { DGNode } from "../../types";
import type { ContainerStatsData } from "../../types/stats";

interface Props {
  nodes: DGNode[];
  statsMap: Map<string, ContainerStatsData>;
}

function useIsNarrow(breakpoint = 900): boolean {
  const [narrow, setNarrow] = useState(() => window.innerWidth < breakpoint);
  useEffect(() => {
    const mql = window.matchMedia(`(max-width: ${breakpoint - 1}px)`);
    const handler = (e: MediaQueryListEvent) => setNarrow(e.matches);
    mql.addEventListener("change", handler);
    return () => mql.removeEventListener("change", handler);
  }, [breakpoint]);
  return narrow;
}

export const Dashboard = memo(function Dashboard({ nodes, statsMap }: Props) {
  const { theme } = useTheme();
  const [timeRange, setTimeRange] = useState<TimeRange>("1h");
  const { data: historyData } = useStatsHistory(timeRange);
  const narrow = useIsNarrow();

  const cols4 = narrow ? "1fr" : "repeat(4, 1fr)";
  const cols2 = narrow ? "1fr" : "1fr 1fr";

  return (
    <div style={{
      position: "absolute",
      inset: 0,
      top: 50,
      overflowY: "auto",
      background: theme.canvasBg,
      fontFamily: "system-ui, -apple-system, sans-serif",
    }}>
      <div style={{ maxWidth: 1440, margin: "0 auto", padding: "20px 24px 32px" }}>

        {/* Header row */}
        <div style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          marginBottom: 20,
        }}>
          <span style={{ fontSize: 14, fontWeight: 600, color: theme.nodeText }}>Dashboard</span>
          <TimeRangeSelector value={timeRange} onChange={setTimeRange} />
        </div>

        {/* Row 1: 4 summary cards — equal height */}
        <div style={{ display: "grid", gridTemplateColumns: cols4, gap: 12, marginBottom: 12 }}>
          <StatusSummaryCard nodes={nodes} />
          <HostInfoCard />
          <DiskUsageCard />
          <ImagesCard />
        </div>

        {/* Row 2–3: Charts in 2-col grid */}
        <div style={{ display: "grid", gridTemplateColumns: cols2, gap: 12, marginBottom: 12 }}>
          <ResourceChart title="CPU Usage" metric="cpu" data={historyData} />
          <ResourceChart title="Memory Usage" metric="mem" data={historyData} />
          <ResourceChart title="Network I/O" metric="netIO" data={historyData} />
          <ResourceChart title="Disk I/O" metric="diskIO" data={historyData} />
        </div>

        {/* Row 4–5: Tables and lists */}
        <div style={{ display: "grid", gridTemplateColumns: cols2, gap: 12 }}>
          <TopConsumersCard statsMap={statsMap} />
          <AlertsCard nodes={nodes} statsMap={statsMap} />
          <ComposeProjectsCard nodes={nodes} />
          <EventTimelineCard />
        </div>
      </div>
    </div>
  );
});
