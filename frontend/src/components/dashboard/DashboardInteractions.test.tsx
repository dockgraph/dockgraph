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
    expect(onStatusFilter).toHaveBeenCalledWith("running");
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

describe("TopConsumersCard interactions", () => {
  it("inspects the container when a row is clicked", () => {
    const onInspect = vi.fn();
    const statsMap = new Map<string, ContainerStatsData>([["web", makeStats()]]);
    render(<TopConsumersCard statsMap={statsMap} onInspect={onInspect} />);
    fireEvent.click(screen.getByText("web"));
    expect(onInspect).toHaveBeenCalledWith("container:web");
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
