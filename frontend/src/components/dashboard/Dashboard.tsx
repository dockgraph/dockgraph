import { useState, useEffect, useMemo, memo } from "react";
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
import { SwarmNodesCard } from "./SwarmNodesCard";
import { SegmentedToggle } from "../GroupByToggle";
import { useStatsHistory, type TimeRange, type StatsHistoryData, type HistoryScope } from "../../hooks/useStatsHistory";
import { useSystemInfo } from "../../hooks/useSystemInfo";
import { STANDALONE_STACK, isWorkload, showsSwarmWorkloads, withoutCoveredTasks } from "../../utils/stack";
import type { ResourceTab } from "../table/TableView";
import type { DGNode } from "../../types";
import type { ContainerStatsData } from "../../types/stats";

interface Props {
  nodes: DGNode[];
  statsMap: Map<string, ContainerStatsData>;
  /** Open the table view's `tab` filtered to a single status. */
  onStatusFilter: (status: string, tab: ResourceTab) => void;
  /** Open the table view on a specific resource subtab. */
  onResourceTab: (tab: ResourceTab) => void;
  /** Open the detail panel for the given graph node id. */
  onInspect: (nodeId: string) => void;
  /** Active stack scope (null = all). `nodes` are already scoped to it. */
  stack?: string | null;
  /** Scope the UI to a stack/project (from the projects card). */
  onSelectStack?: (stack: string) => void;
}

/**
 * The workload history to chart. The backend can only filter history by a
 * named stack, so for the standalone scope keep the series of the (already
 * scoped) workloads client-side. Then plot each workload once: a swarm
 * service's aggregate series stands for its task series.
 */
function scopeHistory(data: StatsHistoryData | null, nodes: DGNode[], stack: string | null | undefined): StatsHistoryData | null {
  if (!data) return data;
  let containers = data.containers;
  if (stack === STANDALONE_STACK) {
    const names = new Set(nodes.filter(isWorkload).map((n) => n.name));
    containers = Object.fromEntries(Object.entries(containers).filter(([name]) => names.has(name)));
  }
  return { ...data, containers: withoutCoveredTasks(containers) };
}

const HISTORY_SCOPES = [
  { key: "workloads", label: "Workload" },
  { key: "nodes", label: "Node" },
] as const;

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

export const Dashboard = memo(function Dashboard({ nodes, statsMap, onStatusFilter, onResourceTab, onInspect, stack, onSelectStack }: Props) {
  const { theme } = useTheme();
  const [timeRange, setTimeRange] = useState<TimeRange>("1h");
  const { data: systemInfo } = useSystemInfo();
  const swarm = systemInfo?.mode === "swarm";
  const swarmWorkloads = showsSwarmWorkloads(nodes, swarm, stack);
  // Charts plot workloads, or one series per swarm node. Node history isn't
  // per stack, so a selected stack forces the workload view.
  const [historyScope, setHistoryScope] = useState<HistoryScope>("workloads");
  const effectiveScope: HistoryScope = swarm && !stack ? historyScope : "workloads";
  const serverStack = stack && stack !== STANDALONE_STACK ? stack : null;
  const { data: rawHistory } = useStatsHistory(timeRange, serverStack, effectiveScope);
  const historyData = useMemo(
    () => (effectiveScope === "nodes" ? rawHistory : scopeHistory(rawHistory, nodes, stack)),
    [rawHistory, nodes, stack, effectiveScope],
  );
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
      fontFamily: "var(--dg-font-ui)",
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
          <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
            {swarm && (
              <SegmentedToggle
                label="By"
                options={HISTORY_SCOPES}
                value={effectiveScope}
                onChange={setHistoryScope}
                disabled={!!stack}
                title={stack ? "Per-node history covers all stacks; clear the stack to plot nodes" : "Plot the charts per workload or per swarm node"}
              />
            )}
            <TimeRangeSelector value={timeRange} onChange={setTimeRange} />
          </div>
        </div>

        {/* Row 1: 4 summary cards — equal height */}
        <div style={{ display: "grid", gridTemplateColumns: cols4, gap: 12, marginBottom: 12 }}>
          <StatusSummaryCard nodes={nodes} swarm={swarmWorkloads} onStatusFilter={onStatusFilter} onResourceTab={onResourceTab} />
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
          <TopConsumersCard statsMap={statsMap} onInspect={onInspect} />
          <AlertsCard nodes={nodes} statsMap={statsMap} onInspect={onInspect} />
          <ComposeProjectsCard nodes={nodes} swarm={swarm} onSelectStack={onSelectStack} />
          <EventTimelineCard nodes={nodes} onInspect={onInspect} />
          {swarm && (
            <div style={{ gridColumn: "1 / -1" }}>
              <SwarmNodesCard nodes={nodes} statsMap={statsMap} onInspect={onInspect} />
            </div>
          )}
        </div>
      </div>
    </div>
  );
});
