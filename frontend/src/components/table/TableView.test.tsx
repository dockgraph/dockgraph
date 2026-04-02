// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from "vitest";
import { render, screen, fireEvent, cleanup } from "@testing-library/react";

afterEach(() => cleanup());

vi.mock("../../theme", () => ({
  useTheme: () => ({
    theme: {
      canvasBg: "#0f172a",
      panelBg: "#1e293b",
      panelBorder: "#334155",
      panelText: "#94a3b8",
      nodeText: "#e2e8f0",
      nodeSubtext: "#64748b",
    },
    toggle: vi.fn(),
  }),
}));

import { TableView } from "./TableView";
import type { DGNode, DGEdge } from "../../types";
import type { ContainerStatsData } from "../../types/stats";

const nodes: DGNode[] = [
  { id: "container:api", type: "container", name: "api", image: "node:22", status: "running", networkId: "network:app" },
  { id: "network:app", type: "network", name: "app-network", driver: "bridge" },
  { id: "volume:data", type: "volume", name: "pgdata", driver: "local" },
];

const edges: DGEdge[] = [];
const statsMap = new Map<string, ContainerStatsData>();

describe("TableView", () => {
  it("renders resource tabs", () => {
    render(<TableView nodes={nodes} edges={edges} statsMap={statsMap} matchingNodeIds={null} selectedNodeId={null} onRowClick={() => {}} />);
    expect(screen.getByText("Containers")).toBeDefined();
    expect(screen.getByText("Networks")).toBeDefined();
    expect(screen.getByText("Volumes")).toBeDefined();
  });

  it("shows containers tab by default", () => {
    render(<TableView nodes={nodes} edges={edges} statsMap={statsMap} matchingNodeIds={null} selectedNodeId={null} onRowClick={() => {}} />);
    expect(screen.getByText("api")).toBeDefined();
  });

  it("switches to networks tab", () => {
    render(<TableView nodes={nodes} edges={edges} statsMap={statsMap} matchingNodeIds={null} selectedNodeId={null} onRowClick={() => {}} />);
    fireEvent.click(screen.getByText("Networks"));
    expect(screen.getByText("app-network")).toBeDefined();
  });

  it("switches to volumes tab", () => {
    render(<TableView nodes={nodes} edges={edges} statsMap={statsMap} matchingNodeIds={null} selectedNodeId={null} onRowClick={() => {}} />);
    fireEvent.click(screen.getByText("Volumes"));
    expect(screen.getByText("pgdata")).toBeDefined();
  });
});
