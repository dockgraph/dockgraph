import { useEffect, useRef, memo } from "react";
import { useTheme } from "../../theme";
import { DashboardCard } from "./DashboardCard";
import { formatBytes, formatPercent } from "../../utils/format";
import type { StatsHistoryData } from "../../hooks/useStatsHistory";
import uPlot from "uplot";
import "uplot/dist/uPlot.min.css";

type Metric = "cpu" | "mem" | "netIO" | "diskIO";

interface Props {
  title: string;
  metric: Metric;
  data: StatsHistoryData | null;
}

const CHART_COLORS = [
  "#3b82f6", "#22c55e", "#f59e0b", "#ef4444", "#8b5cf6",
  "#ec4899", "#06b6d4", "#f97316", "#84cc16", "#6366f1",
];

function hashCode(str: string): number {
  let hash = 0;
  for (let i = 0; i < str.length; i++) {
    hash = ((hash << 5) - hash + str.charCodeAt(i)) | 0;
  }
  return Math.abs(hash);
}

function colorForName(name: string): string {
  return CHART_COLORS[hashCode(name) % CHART_COLORS.length];
}

function buildSeries(
  metric: Metric,
  data: StatsHistoryData,
): { uData: uPlot.AlignedData; seriesOpts: uPlot.Series[] } {
  const names = Object.keys(data.containers);
  const seriesOpts: uPlot.Series[] = [{}];
  const values: (number | null)[][] = [];

  names.forEach((name) => {
    const cs = data.containers[name];
    let vals: number[];

    switch (metric) {
      case "cpu":
        vals = cs.cpu;
        break;
      case "mem":
        vals = cs.mem.map(Number);
        break;
      case "netIO":
        vals = cs.netRx.map((rx, j) => Number(rx) + Number(cs.netTx[j]));
        break;
      case "diskIO":
        vals = cs.blockRead.map((r, j) => Number(r) + Number(cs.blockWrite[j]));
        break;
    }

    values.push(vals);
    seriesOpts.push({
      label: name,
      stroke: colorForName(name),
      width: 1.5,
      fill: metric === "netIO" || metric === "diskIO" ? colorForName(name) + "20" : undefined,
    });
  });

  return {
    uData: [data.timestamps, ...values] as uPlot.AlignedData,
    seriesOpts,
  };
}

export const ResourceChart = memo(function ResourceChart({ title, metric, data }: Props) {
  const { theme } = useTheme();
  const containerRef = useRef<HTMLDivElement>(null);
  const plotRef = useRef<uPlot | null>(null);

  useEffect(() => {
    if (!containerRef.current || !data || data.timestamps.length === 0) {
      return;
    }

    const names = Object.keys(data.containers);
    if (names.length === 0) return;

    const { uData, seriesOpts } = buildSeries(metric, data);

    const valueFn = metric === "cpu"
      ? (v: number) => formatPercent(v)
      : (v: number) => formatBytes(v);

    const opts: uPlot.Options = {
      width: containerRef.current.clientWidth,
      height: 200,
      series: seriesOpts,
      axes: [
        { stroke: theme.nodeSubtext, grid: { stroke: theme.panelBorder } },
        {
          stroke: theme.nodeSubtext,
          grid: { stroke: theme.panelBorder },
          values: (_u: uPlot, vals: number[]) => vals.map(v => valueFn(v)),
        },
      ],
      cursor: { show: true },
      legend: { show: true },
      scales: { x: { time: true } },
    };

    plotRef.current?.destroy();
    plotRef.current = new uPlot(opts, uData, containerRef.current);

    return () => {
      plotRef.current?.destroy();
      plotRef.current = null;
    };
  }, [data, metric, theme]);

  useEffect(() => {
    if (!containerRef.current) return;
    const el = containerRef.current;
    const observer = new ResizeObserver(entries => {
      const entry = entries[0];
      if (entry && plotRef.current) {
        plotRef.current.setSize({ width: entry.contentRect.width, height: 200 });
      }
    });
    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  return (
    <DashboardCard title={title}>
      <div ref={containerRef} style={{ width: "100%", minHeight: 200 }}>
        {(!data || data.timestamps.length === 0) && (
          <div style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            height: 200,
            fontSize: 12,
            color: theme.nodeSubtext,
          }}>
            Collecting data...
          </div>
        )}
      </div>
    </DashboardCard>
  );
});
