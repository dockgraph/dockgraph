// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from "vitest";
import { render, screen, cleanup } from "@testing-library/react";
import { ResourceChart } from "./ResourceChart";
import type { StatsHistoryData } from "../../hooks/useStatsHistory";

afterEach(() => cleanup());

// jsdom has no ResizeObserver; the chart sets one up on mount.
globalThis.ResizeObserver = class {
  observe() {}
  unobserve() {}
  disconnect() {}
};

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

// uPlot draws to a canvas that jsdom can't render; the empty states never
// instantiate it, but mocking keeps the module import side-effect-free.
vi.mock("uplot", () => ({ default: vi.fn(() => ({ destroy: vi.fn(), setSize: vi.fn() })) }));

const emptyData: StatsHistoryData = { range: "1h", resolution: 3, timestamps: [], containers: {} };

describe("ResourceChart empty states", () => {
  it("shows 'Waiting for data' while the first response is in flight", () => {
    render(<ResourceChart title="CPU Usage" metric="cpu" data={null} />);
    expect(screen.getByText("Waiting for data")).toBeTruthy();
    expect(screen.queryByText("No data available")).toBeNull();
  });

  it("shows 'No data available' once a response arrives with no series", () => {
    render(<ResourceChart title="CPU Usage" metric="cpu" data={emptyData} />);
    expect(screen.getByText("No data available")).toBeTruthy();
    expect(screen.queryByText("Waiting for data")).toBeNull();
  });
});
