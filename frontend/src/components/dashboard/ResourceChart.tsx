import { useEffect, useRef, memo, useMemo } from "react";
import { useTheme, type Theme } from "../../theme";
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
  "#64b5f6", "#4ade80", "#fbbf24", "#f87171", "#c084fc",
  "#f472b6", "#22d3ee", "#fb923c", "#a3e635", "#818cf8",
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

/** CSS overrides for uPlot to match the current theme. Re-injected on theme change. */
const CHART_CSS_ID = "dg-uplot-overrides";
function injectChartCSS(theme: Theme) {
  let style = document.getElementById(CHART_CSS_ID) as HTMLStyleElement | null;
  if (!style) {
    style = document.createElement("style");
    style.id = CHART_CSS_ID;
    document.head.appendChild(style);
  }
  style.textContent = `
    .dg-chart .u-legend { font-size: 10px; padding: 4px 0 0; }
    .dg-chart .u-legend .u-series { padding: 1px 6px; }
    .dg-chart .u-legend .u-label { color: ${theme.nodeSubtext}; }
    .dg-chart .u-legend .u-value { color: ${theme.nodeText}; font-family: monospace; font-size: 10px; }
    .dg-chart .u-legend .u-series > * { vertical-align: middle; }
    .dg-chart .u-legend .u-marker { width: 8px !important; height: 3px !important; border-radius: 1px !important; }
    .dg-chart .u-cursor-x,
    .dg-chart .u-cursor-y { border-color: ${theme.panelBorder} !important; }
    .dg-chart .u-select { background: rgba(59,130,246,0.08) !important; }
  `;
}

const CHART_HEIGHT = 160;

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

    const color = colorForName(name);
    values.push(vals);
    seriesOpts.push({
      label: name,
      stroke: color,
      width: 1.5,
      fill: (metric === "netIO" || metric === "diskIO") ? color + "18" : undefined,
      points: { show: false },
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

  // Inject/update CSS on theme change.
  useEffect(() => { injectChartCSS(theme); }, [theme]);

  const hasData = useMemo(
    () => data != null && data.timestamps.length > 0 && Object.keys(data.containers).length > 0,
    [data],
  );

  useEffect(() => {
    if (!containerRef.current || !hasData || !data) return;

    const { uData, seriesOpts } = buildSeries(metric, data);

    const valueFn = metric === "cpu"
      ? (v: number) => formatPercent(v)
      : (v: number) => formatBytes(v);

    const opts: uPlot.Options = {
      width: containerRef.current.clientWidth,
      height: CHART_HEIGHT,
      padding: [8, 8, 0, 0],
      series: seriesOpts,
      axes: [
        {
          stroke: theme.nodeSubtext,
          grid: { stroke: theme.panelBorder, width: 1 },
          ticks: { stroke: theme.panelBorder, width: 1 },
          font: "10px system-ui",
          gap: 4,
        },
        {
          stroke: theme.nodeSubtext,
          grid: { stroke: theme.panelBorder, width: 1 },
          ticks: { show: false },
          font: "10px monospace",
          gap: 4,
          size: 54,
          values: (_u: uPlot, vals: number[]) => vals.map(v => valueFn(v)),
        },
      ],
      cursor: {
        show: true,
        x: true,
        y: false,
        drag: { x: false, y: false },
      },
      legend: { show: true, live: true },
      scales: { x: { time: true } },
    };

    plotRef.current?.destroy();
    plotRef.current = new uPlot(opts, uData, containerRef.current);

    return () => {
      plotRef.current?.destroy();
      plotRef.current = null;
    };
  }, [data, metric, theme, hasData]);

  useEffect(() => {
    if (!containerRef.current) return;
    const el = containerRef.current;
    const observer = new ResizeObserver(entries => {
      const entry = entries[0];
      if (entry && plotRef.current) {
        plotRef.current.setSize({ width: entry.contentRect.width, height: CHART_HEIGHT });
      }
    });
    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  return (
    <DashboardCard title={title}>
      <div ref={containerRef} className="dg-chart" style={{ width: "100%", minHeight: CHART_HEIGHT }}>
        {!hasData && (
          <div style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            height: CHART_HEIGHT,
            fontSize: 11,
            color: theme.nodeSubtext,
            opacity: 0.6,
          }}>
            Waiting for data...
          </div>
        )}
      </div>
    </DashboardCard>
  );
});
