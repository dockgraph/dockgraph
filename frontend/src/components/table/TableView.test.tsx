// @vitest-environment jsdom
import { useState } from "react";
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

import { TableView, type ResourceTab } from "./TableView";
import type { DGNode, DGEdge } from "../../types";
import type { ContainerStatsData } from "../../types/stats";

const nodes: DGNode[] = [
  { id: "container:api", type: "container", name: "api", image: "node:22", status: "running", networkId: "network:app" },
  { id: "network:app", type: "network", name: "app-network", driver: "bridge" },
  { id: "volume:data", type: "volume", name: "pgdata", driver: "local" },
];

const edges: DGEdge[] = [];
const statsMap = new Map<string, ContainerStatsData>();

/** Wrap TableView with local tab state so it behaves as a controlled component. */
function Harness({ initialTab = "containers", selectedNodeId = null }: { initialTab?: ResourceTab; selectedNodeId?: string | null }) {
  const [tab, setTab] = useState<ResourceTab>(initialTab);
  return (
    <TableView
      nodes={nodes}
      edges={edges}
      statsMap={statsMap}
      matchingNodeIds={null}
      selectedNodeId={selectedNodeId}
      onRowClick={() => {}}
      activeTab={tab}
      onTabChange={setTab}
    />
  );
}

describe("TableView", () => {
  it("renders resource tabs", () => {
    render(<Harness />);
    expect(screen.getByText("Containers")).toBeDefined();
    expect(screen.getByText("Networks")).toBeDefined();
    expect(screen.getByText("Volumes")).toBeDefined();
  });

  it("shows containers tab by default", () => {
    render(<Harness />);
    expect(screen.getByText("api")).toBeDefined();
  });

  it("switches to networks tab", () => {
    render(<Harness />);
    fireEvent.click(screen.getByText("Networks"));
    expect(screen.getByText("app-network")).toBeDefined();
  });

  it("switches to volumes tab", () => {
    render(<Harness />);
    fireEvent.click(screen.getByText("Volumes"));
    expect(screen.getByText("pgdata")).toBeDefined();
  });

  it("opens the subtab requested via initial tab state", () => {
    render(<Harness initialTab="volumes" />);
    expect(screen.getByText("pgdata")).toBeDefined();
  });

  it("follows the selection to its resource subtab", () => {
    render(<Harness selectedNodeId="network:app" />);
    expect(screen.getByText("app-network")).toBeDefined();
  });
});
