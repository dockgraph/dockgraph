// @vitest-environment jsdom
import { describe, it, expect } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useTableGrouping } from "./useTableGrouping";
import type { DGNode } from "../types";

const makeNode = (overrides: Partial<DGNode> & { id: string; name: string }): DGNode => ({
  type: "container",
  ...overrides,
});

describe("useTableGrouping", () => {
  const nodes: DGNode[] = [
    makeNode({ id: "c:a", name: "api", networkId: "network:frontend" }),
    makeNode({ id: "c:b", name: "db", networkId: "network:backend" }),
    makeNode({ id: "c:c", name: "cache", networkId: "network:frontend" }),
    makeNode({ id: "c:d", name: "worker", status: "exited" }),
  ];

  it("defaults to grouping by compose project", () => {
    const { result } = renderHook(() => useTableGrouping(nodes));
    expect(result.current.groupBy).toBe("compose");
  });

  it("groups by network", () => {
    const { result } = renderHook(() => useTableGrouping(nodes));
    act(() => result.current.setGroupBy("network"));
    const groups = result.current.groups;
    expect(groups.length).toBe(3);
    const names = groups.map((g) => g.key);
    expect(names).toContain("network:frontend");
    expect(names).toContain("network:backend");
  });

  it("groups by status", () => {
    const { result } = renderHook(() => useTableGrouping(nodes));
    act(() => result.current.setGroupBy("status"));
    const groups = result.current.groups;
    const keys = groups.map((g) => g.key);
    expect(keys).toContain("exited");
  });

  it("returns flat list when groupBy is none", () => {
    const { result } = renderHook(() => useTableGrouping(nodes));
    act(() => result.current.setGroupBy("none"));
    expect(result.current.groups.length).toBe(1);
    expect(result.current.groups[0].items.length).toBe(4);
  });

  it("toggles group collapsed state", () => {
    const { result } = renderHook(() => useTableGrouping(nodes));
    const key = result.current.groups[0].key;
    expect(result.current.isCollapsed(key)).toBe(false);
    act(() => result.current.toggleCollapsed(key));
    expect(result.current.isCollapsed(key)).toBe(true);
    act(() => result.current.toggleCollapsed(key));
    expect(result.current.isCollapsed(key)).toBe(false);
  });

  it("accepts a custom default groupBy", () => {
    const { result } = renderHook(() => useTableGrouping(nodes, "network"));
    expect(result.current.groupBy).toBe("network");
  });

  it("groups by driver", () => {
    const driverNodes: DGNode[] = [
      makeNode({ id: "n:a", name: "net-a", type: "network", driver: "bridge" }),
      makeNode({ id: "n:b", name: "net-b", type: "network", driver: "overlay" }),
      makeNode({ id: "n:c", name: "net-c", type: "network", driver: "bridge" }),
    ];
    const { result } = renderHook(() => useTableGrouping(driverNodes, "driver"));
    const groups = result.current.groups;
    expect(groups.length).toBe(2);
    const keys = groups.map((g) => g.key);
    expect(keys).toContain("bridge");
    expect(keys).toContain("overlay");
    const bridgeGroup = groups.find((g) => g.key === "bridge");
    expect(bridgeGroup?.items.length).toBe(2);
  });
});
