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

import { NetworkTable } from "./NetworkTable";
import type { DGNode } from "../../types";

const nodes: DGNode[] = [
  { id: "network:app", type: "network", name: "app-network", driver: "bridge" },
  { id: "network:db", type: "network", name: "db-network", driver: "overlay" },
];

const allNodes: DGNode[] = [
  ...nodes,
  { id: "container:a", type: "container", name: "a", networkId: "network:app" },
  { id: "container:b", type: "container", name: "b", networkId: "network:app" },
  { id: "container:c", type: "container", name: "c", networkId: "network:db" },
];

describe("NetworkTable", () => {
  it("renders column headers", () => {
    render(<NetworkTable nodes={nodes} allNodes={allNodes} edges={[]} selectedNodeId={null} onRowClick={() => {}} />);
    const buttons = screen.getAllByRole("button");
    const labels = buttons.map((b) => b.textContent?.trim());
    expect(labels.some((l) => l?.startsWith("Name"))).toBe(true);
    expect(labels).toContain("Driver");
  });

  it("renders all network names", () => {
    render(<NetworkTable nodes={nodes} allNodes={allNodes} edges={[]} selectedNodeId={null} onRowClick={() => {}} />);
    expect(screen.getByText("app-network")).toBeDefined();
    expect(screen.getByText("db-network")).toBeDefined();
  });

  it("shows correct container count", () => {
    render(<NetworkTable nodes={nodes} allNodes={allNodes} edges={[]} selectedNodeId={null} onRowClick={() => {}} />);
    expect(screen.getByText("2")).toBeDefined();
    expect(screen.getByText("1")).toBeDefined();
  });

  it("calls onRowClick when a row is clicked", () => {
    const onClick = vi.fn();
    render(<NetworkTable nodes={nodes} allNodes={allNodes} edges={[]} selectedNodeId={null} onRowClick={onClick} />);
    fireEvent.click(screen.getByText("app-network"));
    expect(onClick).toHaveBeenCalledWith("network:app");
  });

  it("includes secondary network edges in container count", () => {
    const edges = [
      { source: "container:d", target: "network:db", type: "secondary_network" },
    ];
    render(<NetworkTable nodes={nodes} allNodes={allNodes} edges={edges} selectedNodeId={null} onRowClick={() => {}} />);
    // db-network: 1 primary + 1 secondary = 2
    expect(screen.getAllByText("2").length).toBe(2);
  });
});
