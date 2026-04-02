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

import { VolumeTable } from "./VolumeTable";
import type { DGNode, DGEdge } from "../../types";

const nodes: DGNode[] = [
  { id: "volume:pgdata", type: "volume", name: "pgdata", driver: "local" },
  { id: "volume:redis", type: "volume", name: "redis-data", driver: "local" },
];

const edges: DGEdge[] = [
  { id: "e1", type: "volume_mount", source: "volume:pgdata", target: "container:db", mountPath: "/var/lib/postgresql/data" },
  { id: "e2", type: "volume_mount", source: "volume:pgdata", target: "container:cache", mountPath: "/data" },
];

describe("VolumeTable", () => {
  it("renders column headers", () => {
    render(<VolumeTable nodes={nodes} edges={edges} selectedNodeId={null} onRowClick={() => {}} />);
    const buttons = screen.getAllByRole("button");
    const labels = buttons.map((b) => b.textContent?.trim());
    expect(labels.some((l) => l?.startsWith("Name"))).toBe(true);
    expect(labels).toContain("Driver");
  });

  it("renders all volume names", () => {
    render(<VolumeTable nodes={nodes} edges={edges} selectedNodeId={null} onRowClick={() => {}} />);
    expect(screen.getByText("pgdata")).toBeDefined();
    expect(screen.getByText("redis-data")).toBeDefined();
  });

  it("shows container count from edges", () => {
    render(<VolumeTable nodes={nodes} edges={edges} selectedNodeId={null} onRowClick={() => {}} />);
    expect(screen.getByText("2")).toBeDefined();
    expect(screen.getByText("0")).toBeDefined();
  });

  it("calls onRowClick when a row is clicked", () => {
    const onClick = vi.fn();
    render(<VolumeTable nodes={nodes} edges={edges} selectedNodeId={null} onRowClick={onClick} />);
    fireEvent.click(screen.getByText("pgdata"));
    expect(onClick).toHaveBeenCalledWith("volume:pgdata");
  });
});
