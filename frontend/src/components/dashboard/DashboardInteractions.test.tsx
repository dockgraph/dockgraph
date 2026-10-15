// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from "vitest";
import { render, screen, fireEvent, cleanup } from "@testing-library/react";

afterEach(() => cleanup());

vi.mock("../../theme", () => ({
  useTheme: () => ({
    theme: {
      nodeText: "#e2e8f0",
      nodeSubtext: "#64748b",
      panelBg: "#1e293b",
      panelBorder: "#334155",
      canvasBg: "#0f172a",
      rowHover: "#1c2738",
      accent: "#60a5fa",
    },
    toggle: vi.fn(),
  }),
}));

const mockEvents = vi.fn();
vi.mock("../../hooks/useRecentEvents", () => ({
  useRecentEvents: () => mockEvents(),
}));

import { StatusSummaryCard } from "./StatusSummaryCard";
import { TopConsumersCard } from "./TopConsumersCard";
import { EventTimelineCard } from "./EventTimelineCard";
import { AlertsCard } from "./AlertsCard";
import type { DGNode } from "../../types";
import type { ContainerStatsData } from "../../types/stats";

const containers: DGNode[] = [
  { id: "container:web", type: "container", name: "web", status: "running" },
  { id: "container:db", type: "container", name: "db", status: "exited" },
  { id: "network:app", type: "network", name: "app" },
  { id: "volume:data", type: "volume", name: "data" },
];

function makeStats(): ContainerStatsData {
  return {
    cpuPercent: 5, cpuThrottled: 0, memUsage: 1024, memLimit: 2048,
    netRx: 10, netTx: 20, netRxErrors: 0, netTxErrors: 0,
    blockRead: 0, blockWrite: 0, pids: 3,
  };
}

describe("StatusSummaryCard interactions", () => {
  it("filters the table by status when a status row is clicked", () => {
    const onStatusFilter = vi.fn();
    render(<StatusSummaryCard nodes={containers} onStatusFilter={onStatusFilter} onResourceTab={vi.fn()} />);
    fireEvent.click(screen.getByText("Running"));
    expect(onStatusFilter).toHaveBeenCalledWith("running", "containers");
  });

  it("opens the matching subtab when a resource total is clicked", () => {
    const onResourceTab = vi.fn();
    render(<StatusSummaryCard nodes={containers} onStatusFilter={vi.fn()} onResourceTab={onResourceTab} />);
    fireEvent.click(screen.getByText("Networks"));
    expect(onResourceTab).toHaveBeenCalledWith("networks");
    fireEvent.click(screen.getByText("Volumes"));
    expect(onResourceTab).toHaveBeenCalledWith("volumes");
  });
});

describe("StatusSummaryCard in swarm mode", () => {
  const swarmNodes: DGNode[] = [
    { id: "service:shop_web", type: "service", name: "shop_web", status: "running" },
    { id: "service:shop_api", type: "service", name: "shop_api", status: "degraded" },
    { id: "service:shop_db", type: "service", name: "shop_db", status: "running" },
    { id: "container:lone", type: "container", name: "lone", status: "running" }, // not a swarm task
    { id: "network:app", type: "network", name: "app" },
  ];
  const row = (label: string) => screen.getByText(label).closest("button")!;

  it("counts services, not containers", () => {
    render(<StatusSummaryCard nodes={swarmNodes} swarm onStatusFilter={vi.fn()} onResourceTab={vi.fn()} />);
    expect(screen.getByText("Services")).toBeTruthy();
    expect(row("Running").textContent).toContain("2");
    expect(row("Degraded").textContent).toContain("1");
    expect(screen.queryByText("Exited")).toBeNull();
  });

  it("drills into the Services tab", () => {
    const onStatusFilter = vi.fn();
    render(<StatusSummaryCard nodes={swarmNodes} swarm onStatusFilter={onStatusFilter} onResourceTab={vi.fn()} />);
    fireEvent.click(screen.getByText("Degraded"));
    expect(onStatusFilter).toHaveBeenCalledWith("degraded", "services");
  });

  it("keeps standalone containers reachable from the totals", () => {
    const onResourceTab = vi.fn();
    render(<StatusSummaryCard nodes={swarmNodes} swarm onStatusFilter={vi.fn()} onResourceTab={onResourceTab} />);
    fireEvent.click(screen.getByText("Containers"));
    expect(onResourceTab).toHaveBeenCalledWith("containers");
  });

  it("hides the containers total when there are none", () => {
    const servicesOnly = swarmNodes.filter((n) => n.type !== "container");
    render(<StatusSummaryCard nodes={servicesOnly} swarm onStatusFilter={vi.fn()} onResourceTab={vi.fn()} />);
    expect(screen.queryByText("Containers")).toBeNull();
  });
});

describe("TopConsumersCard interactions", () => {
  it("inspects the container when a row is clicked", () => {
    const onInspect = vi.fn();
    const statsMap = new Map<string, ContainerStatsData>([["web", makeStats()]]);
    render(<TopConsumersCard statsMap={statsMap} onInspect={onInspect} />);
    fireEvent.click(screen.getByText("web"));
    expect(onInspect).toHaveBeenCalledWith("container:web");
  });

  it("does not list per-swarm-node aggregates as consumers", () => {
    const statsMap = new Map<string, ContainerStatsData>([
      ["web", makeStats()],
      ["node:mgr", { ...makeStats(), cpuPercent: 999 }],
    ]);
    render(<TopConsumersCard statsMap={statsMap} onInspect={vi.fn()} />);
    expect(screen.getByText("web")).toBeDefined();
    expect(screen.queryByText("node:mgr")).toBeNull();
  });

  it("lists a swarm service once, not next to its own tasks", () => {
    const id = "dvikfv6mt2qtwncfvburtsurx";
    const statsMap = new Map<string, ContainerStatsData>([
      ["shop_web", { ...makeStats(), cpuPercent: 80 }], // aggregate of both tasks
      [`shop_web.1.${id}`, { ...makeStats(), cpuPercent: 50 }],
      [`shop_web.2.${id}`, { ...makeStats(), cpuPercent: 30 }],
      ["db", makeStats()],
    ]);
    render(<TopConsumersCard statsMap={statsMap} onInspect={vi.fn()} />);
    expect(screen.getByText("shop_web")).toBeDefined();
    expect(screen.getByText("db")).toBeDefined();
    expect(screen.queryByText(`shop_web.1.${id}`)).toBeNull();
    expect(screen.queryByText(`shop_web.2.${id}`)).toBeNull();
  });
});

describe("EventTimelineCard interactions", () => {
  it("inspects the referenced resource for a resolvable event", () => {
    mockEvents.mockReturnValue({
      data: { events: [{ timestamp: "2026-06-10T10:00:00Z", action: "start", type: "container", name: "web" }] },
    });
    const onInspect = vi.fn();
    render(<EventTimelineCard nodes={containers} onInspect={onInspect} />);
    fireEvent.click(screen.getByText("web"));
    expect(onInspect).toHaveBeenCalledWith("container:web");
  });

  it("does not make image events (with no graph node) clickable", () => {
    mockEvents.mockReturnValue({
      data: { events: [{ timestamp: "2026-06-10T10:00:00Z", action: "pull", type: "image", name: "nginx:latest" }] },
    });
    const onInspect = vi.fn();
    render(<EventTimelineCard nodes={containers} onInspect={onInspect} />);
    fireEvent.click(screen.getByText("nginx:latest"));
    expect(onInspect).not.toHaveBeenCalled();
  });
});

describe("AlertsCard interactions", () => {
  it("inspects the container an alert refers to", () => {
    const onInspect = vi.fn();
    // The exited "db" container produces a "Container exited" alert.
    const statsMap = new Map<string, ContainerStatsData>();
    render(<AlertsCard nodes={containers} statsMap={statsMap} onInspect={onInspect} />);
    fireEvent.click(screen.getByText("Container exited"));
    expect(onInspect).toHaveBeenCalledWith("container:db");
  });

  it("inspects the swarm service an alert refers to", () => {
    const onInspect = vi.fn();
    const services: DGNode[] = [
      { id: "service:shop_api", type: "service", name: "shop_api", status: "degraded", service: { replicas: { running: 1, desired: 2 } } },
    ];
    render(<AlertsCard nodes={services} statsMap={new Map()} onInspect={onInspect} />);
    fireEvent.click(screen.getByText("Service degraded: 1/2 running"));
    expect(onInspect).toHaveBeenCalledWith("service:shop_api");
  });
});
