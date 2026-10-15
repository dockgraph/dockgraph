// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from "vitest";
import { render, cleanup } from "@testing-library/react";
import type { StatsHistoryData, ContainerTimeSeries } from "../../hooks/useStatsHistory";

afterEach(() => cleanup());

// jsdom has no matchMedia; the dashboard watches it for its narrow layout.
window.matchMedia = (query: string) =>
  ({ matches: false, media: query, addEventListener: () => {}, removeEventListener: () => {} }) as unknown as MediaQueryList;

vi.mock("../../theme", () => ({
  useTheme: () => ({ theme: { nodeText: "#e2e8f0", canvasBg: "#0f172a" }, toggle: vi.fn() }),
}));

const history = vi.fn();
vi.mock("../../hooks/useStatsHistory", () => ({ useStatsHistory: () => ({ data: history() }) }));
vi.mock("../../hooks/useSystemInfo", () => ({ useSystemInfo: () => ({ data: { mode: "swarm" } }) }));

// Record the series each chart is given; the other cards fetch on their own
// and aren't under test.
const charted = vi.fn();
vi.mock("./ResourceChart", () => ({
  ResourceChart: ({ data }: { data: StatsHistoryData | null }) => {
    charted(data ? Object.keys(data.containers) : null);
    return null;
  },
}));
vi.mock("./StatusSummaryCard", () => ({ StatusSummaryCard: () => null }));
vi.mock("./HostInfoCard", () => ({ HostInfoCard: () => null }));
vi.mock("./DiskUsageCard", () => ({ DiskUsageCard: () => null }));
vi.mock("./ImagesCard", () => ({ ImagesCard: () => null }));
vi.mock("./TopConsumersCard", () => ({ TopConsumersCard: () => null }));
vi.mock("./AlertsCard", () => ({ AlertsCard: () => null }));
vi.mock("./ComposeProjectsCard", () => ({ ComposeProjectsCard: () => null }));
vi.mock("./EventTimelineCard", () => ({ EventTimelineCard: () => null }));
vi.mock("./SwarmNodesCard", () => ({ SwarmNodesCard: () => null }));
vi.mock("../GroupByToggle", () => ({ SegmentedToggle: () => null }));

import { Dashboard } from "./Dashboard";

const id = "dvikfv6mt2qtwncfvburtsurx"; // 25-char swarm ID
const series: ContainerTimeSeries = { cpu: [1], mem: [1], netRx: [0], netTx: [0], blockRead: [0], blockWrite: [0] };

describe("Dashboard history charts", () => {
  it("plots each swarm service once, not its aggregate plus every task", () => {
    history.mockReturnValue({
      range: "1h", resolution: 3, timestamps: [1],
      containers: {
        shop_web: series, // service aggregate
        [`shop_web.1.${id}`]: series, // its tasks: covered by the aggregate
        [`shop_web.2.${id}`]: series,
        [`orphan_svc.1.${id}`]: series, // task without an aggregate: kept
        db: series,
      },
    });
    render(
      <Dashboard nodes={[]} statsMap={new Map()} onStatusFilter={vi.fn()} onResourceTab={vi.fn()} onInspect={vi.fn()} />,
    );
    expect(charted).toHaveBeenCalled();
    for (const [keys] of charted.mock.calls) {
      expect(keys).toEqual(["shop_web", `orphan_svc.1.${id}`, "db"]);
    }
  });
});
