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

  const columns = narrow ? "1fr" : "repeat(4, 1fr)";
  const wideSpan = narrow ? "span 1" : "span 2";

  return (
    <div style={{
      position: "absolute",
      inset: 0,
      top: 50,
      overflowY: "auto",
      background: theme.canvasBg,
      padding: "16px 24px",
    }}>
      <div style={{ display: "flex", justifyContent: "flex-end", marginBottom: 16 }}>
        <TimeRangeSelector value={timeRange} onChange={setTimeRange} />
      </div>

      <div style={{ display: "grid", gridTemplateColumns: columns, gap: 16 }}>
        {/* Row 1: Summary cards */}
        <StatusSummaryCard nodes={nodes} />
        <HostInfoCard />
        <DiskUsageCard />
        <ImagesCard />

        {/* Row 2: CPU and Memory charts */}
        <div style={{ gridColumn: wideSpan }}>
          <ResourceChart title="CPU Usage" metric="cpu" data={historyData} />
        </div>
        <div style={{ gridColumn: wideSpan }}>
          <ResourceChart title="Memory Usage" metric="mem" data={historyData} />
        </div>

        {/* Row 3: Network and Disk I/O */}
        <div style={{ gridColumn: wideSpan }}>
          <ResourceChart title="Network I/O" metric="netIO" data={historyData} />
        </div>
        <div style={{ gridColumn: wideSpan }}>
          <ResourceChart title="Disk I/O" metric="diskIO" data={historyData} />
        </div>

        {/* Row 4: Top consumers and alerts */}
        <div style={{ gridColumn: wideSpan }}>
          <TopConsumersCard statsMap={statsMap} />
        </div>
        <div style={{ gridColumn: wideSpan }}>
          <AlertsCard nodes={nodes} statsMap={statsMap} />
        </div>

        {/* Row 5: Compose projects and events */}
        <div style={{ gridColumn: wideSpan }}>
          <ComposeProjectsCard nodes={nodes} />
        </div>
        <div style={{ gridColumn: wideSpan }}>
          <EventTimelineCard />
        </div>
      </div>
    </div>
  );
});
