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
    },
    toggle: vi.fn(),
  }),
}));

import { ContainerRow } from "./ContainerRow";
import type { DGNode } from "../../types";
import type { ContainerStatsData } from "../../types/stats";

const node: DGNode = {
  id: "container:api",
  type: "container",
  name: "api-server",
  image: "node:22-alpine",
  status: "running",
  ports: [{ host: 3000, container: 3000 }],
  networkId: "network:app",
};

const stats: ContainerStatsData = {
  cpuPercent: 2.3,
  cpuThrottled: 0,
  memUsage: 134217728,
  memLimit: 536870912,
  netRx: 1258291,
  netTx: 348160,
  netRxErrors: 0,
  netTxErrors: 0,
  blockRead: 0,
  blockWrite: 0,
  pids: 5,
};

describe("ContainerRow", () => {
  it("renders container name and image", () => {
    render(<ContainerRow node={node} stats={stats} selected={false} onClick={() => {}} gridTemplate="1fr 1fr 1fr 1fr 1fr" />);
    expect(screen.getByText("api-server")).toBeDefined();
    expect(screen.getByText("node:22-alpine")).toBeDefined();
  });

  it("renders status with color dot", () => {
    const { container } = render(
      <ContainerRow node={node} stats={stats} selected={false} onClick={() => {}} gridTemplate="1fr 1fr 1fr 1fr 1fr" />,
    );
    expect(screen.getByText("running")).toBeDefined();
    const dot = container.querySelector("[data-testid='status-dot']") as HTMLElement;
    expect(dot).toBeDefined();
  });

  it("renders stats sub-row with CPU and MEM", () => {
    render(<ContainerRow node={node} stats={stats} selected={false} onClick={() => {}} gridTemplate="1fr 1fr 1fr 1fr 1fr" />);
    expect(screen.getByText(/CPU/)).toBeDefined();
    expect(screen.getByText(/MEM/)).toBeDefined();
  });

  it("shows 'No stats available' when stats is undefined", () => {
    render(<ContainerRow node={node} stats={undefined} selected={false} onClick={() => {}} gridTemplate="1fr 1fr 1fr 1fr 1fr" />);
    expect(screen.getByText("No stats available")).toBeDefined();
  });

  it("calls onClick when row is clicked", () => {
    const onClick = vi.fn();
    render(<ContainerRow node={node} stats={stats} selected={false} onClick={onClick} gridTemplate="1fr 1fr 1fr 1fr 1fr" />);
    fireEvent.click(screen.getByText("api-server"));
    expect(onClick).toHaveBeenCalledWith("container:api");
  });

  it("renders with reduced opacity for exited containers", () => {
    const exited = { ...node, status: "exited" };
    const { container } = render(
      <ContainerRow node={exited} stats={undefined} selected={false} onClick={() => {}} gridTemplate="1fr 1fr 1fr 1fr 1fr" />,
    );
    const wrapper = container.firstElementChild as HTMLElement;
    expect(wrapper.style.opacity).toBe("0.5");
  });
});
