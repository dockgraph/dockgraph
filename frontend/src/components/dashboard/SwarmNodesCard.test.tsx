// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from "vitest";
import { render, screen, fireEvent, cleanup } from "@testing-library/react";
import { ThemeProvider } from "../../theme";
import { SwarmNodesCard } from "./SwarmNodesCard";
import type { DGNode } from "../../types";
import type { ContainerStatsData } from "../../types/stats";

afterEach(() => cleanup());

const stats: ContainerStatsData = {
  cpuPercent: 150, cpuThrottled: 0, memUsage: 1024 ** 3, memLimit: 0,
  netRx: 0, netTx: 0, netRxErrors: 0, netTxErrors: 0, blockRead: 0, blockWrite: 0, pids: 0,
};

const nodes: DGNode[] = [
  {
    id: "swarmnode:wrk", type: "swarmnode", name: "wrk", status: "down",
    swarmNode: { id: "n2", role: "worker", leader: false, availability: "drain", state: "down" },
  },
  {
    id: "swarmnode:mgr", type: "swarmnode", name: "mgr", status: "ready",
    swarmNode: { id: "n1", role: "manager", leader: true, availability: "active", state: "ready", nanoCpus: 2e9, memoryBytes: 4 * 1024 ** 3 },
  },
  {
    id: "service:shop_web", type: "service", name: "shop_web", stack: "shop", status: "running",
    service: {
      replicas: { running: 2, desired: 2 },
      tasks: [
        { id: "a", slot: 1, nodeHostname: "mgr", nodeId: "n1", state: "running", desiredState: "running" },
        { id: "b", slot: 2, nodeHostname: "mgr", nodeId: "n1", state: "running", desiredState: "running" },
      ],
    },
  },
];

describe("SwarmNodesCard", () => {
  it("lists nodes leader first with task counts, capacity and a no-agent row", () => {
    const statsMap = new Map([["node:mgr", stats]]);
    render(<ThemeProvider><SwarmNodesCard nodes={nodes} statsMap={statsMap} /></ThemeProvider>);
    expect(screen.getByText("Swarm Nodes")).toBeDefined();
    const names = screen.getAllByText(/^(mgr|wrk)$/).map((e) => e.textContent);
    expect(names).toEqual(["mgr", "wrk"]);
    expect(screen.getByText("2 tasks")).toBeDefined();
    expect(screen.getByText("0 tasks")).toBeDefined();
    expect(screen.getByText("150.0% / 2 cores")).toBeDefined();
    expect(screen.getByText("no agent")).toBeDefined();
    expect(screen.getByText("drain")).toBeDefined();
    expect(screen.getByText("manager ★")).toBeDefined();
  });

  it("opens the node panel when a row is clicked", () => {
    const onInspect = vi.fn();
    render(<ThemeProvider><SwarmNodesCard nodes={nodes} statsMap={new Map()} onInspect={onInspect} /></ThemeProvider>);
    fireEvent.click(screen.getByText("wrk"));
    expect(onInspect).toHaveBeenCalledWith("swarmnode:wrk");
  });

  it("shows an empty state without swarm nodes", () => {
    render(<ThemeProvider><SwarmNodesCard nodes={[]} statsMap={new Map()} /></ThemeProvider>);
    expect(screen.getByText("No swarm nodes")).toBeDefined();
  });
});
