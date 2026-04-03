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

import { ContainerTable } from "./ContainerTable";
import type { DGNode } from "../../types";
import type { ContainerStatsData } from "../../types/stats";

const nodes: DGNode[] = [
  { id: "container:api", type: "container", name: "api", image: "node:22", status: "running", networkId: "network:frontend", labels: { "com.docker.compose.project": "webapp" } },
  { id: "container:db", type: "container", name: "db", image: "postgres:16", status: "running", networkId: "network:frontend", labels: { "com.docker.compose.project": "webapp" } },
  { id: "container:redis", type: "container", name: "redis", image: "redis:7", status: "exited", networkId: "network:cache", labels: { "com.docker.compose.project": "cache-stack" } },
];

const statsMap = new Map<string, ContainerStatsData>();

describe("ContainerTable", () => {
  it("renders column headers", () => {
    render(<ContainerTable nodes={nodes} statsMap={statsMap} selectedNodeId={null} onRowClick={() => {}} />);
    const buttons = screen.getAllByRole("button");
    const labels = buttons.map((b) => b.textContent?.trim());
    expect(labels.some((l) => l?.startsWith("Name"))).toBe(true);
    expect(labels).toContain("Image");
  });

  it("renders all container names", () => {
    render(<ContainerTable nodes={nodes} statsMap={statsMap} selectedNodeId={null} onRowClick={() => {}} />);
    expect(screen.getByText("api")).toBeDefined();
    expect(screen.getByText("db")).toBeDefined();
    expect(screen.getByText("redis")).toBeDefined();
  });

  it("renders group headers when grouped by compose project", () => {
    render(<ContainerTable nodes={nodes} statsMap={statsMap} selectedNodeId={null} onRowClick={() => {}} />);
    const groupButtons = screen.getAllByRole("button").filter(
      (el) => el.getAttribute("tabindex") === "0",
    );
    const groupLabels = groupButtons.map((b) => b.textContent);
    expect(groupLabels.some((l) => l?.includes("Webapp"))).toBe(true);
    expect(groupLabels.some((l) => l?.includes("Cache-stack"))).toBe(true);
  });

  it("collapses a group when header is clicked", () => {
    render(<ContainerTable nodes={nodes} statsMap={statsMap} selectedNodeId={null} onRowClick={() => {}} />);
    const groupButtons = screen.getAllByRole("button").filter(
      (el) => el.getAttribute("tabindex") === "0",
    );
    const webappGroup = groupButtons.find((b) => b.textContent?.includes("Webapp"))!;
    fireEvent.click(webappGroup);
    expect(screen.queryByText("api")).toBeNull();
    expect(screen.queryByText("db")).toBeNull();
    expect(screen.getByText("redis")).toBeDefined();
  });

  it("calls onRowClick when a container row is clicked", () => {
    const onClick = vi.fn();
    render(<ContainerTable nodes={nodes} statsMap={statsMap} selectedNodeId={null} onRowClick={onClick} />);
    fireEvent.click(screen.getByText("api"));
    expect(onClick).toHaveBeenCalledWith("container:api");
  });
});
