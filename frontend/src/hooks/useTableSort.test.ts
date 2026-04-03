// @vitest-environment jsdom
import { describe, it, expect } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useTableSort } from "./useTableSort";

describe("useTableSort", () => {
  it("starts with the given default column ascending", () => {
    const { result } = renderHook(() => useTableSort("name"));
    expect(result.current.column).toBe("name");
    expect(result.current.direction).toBe("asc");
  });

  it("toggles to descending on same column", () => {
    const { result } = renderHook(() => useTableSort("name"));
    act(() => result.current.toggleSort("name"));
    expect(result.current.column).toBe("name");
    expect(result.current.direction).toBe("desc");
  });

  it("resets to ascending on new column", () => {
    const { result } = renderHook(() => useTableSort("name"));
    act(() => result.current.toggleSort("name")); // desc
    act(() => result.current.toggleSort("status")); // new col → asc
    expect(result.current.column).toBe("status");
    expect(result.current.direction).toBe("asc");
  });

  it("sortItems sorts strings ascending", () => {
    const { result } = renderHook(() => useTableSort("name"));
    const items = [{ name: "zebra" }, { name: "alpha" }, { name: "mango" }];
    const sorted = result.current.sortItems(items, (item) => ({ name: item.name }));
    expect(sorted.map((i) => i.name)).toEqual(["alpha", "mango", "zebra"]);
  });

  it("sortItems sorts strings descending", () => {
    const { result } = renderHook(() => useTableSort("name"));
    act(() => result.current.toggleSort("name")); // desc
    const items = [{ name: "zebra" }, { name: "alpha" }, { name: "mango" }];
    const sorted = result.current.sortItems(items, (item) => ({ name: item.name }));
    expect(sorted.map((i) => i.name)).toEqual(["zebra", "mango", "alpha"]);
  });

  it("sortItems sorts numbers", () => {
    const { result } = renderHook(() => useTableSort("count"));
    const items = [{ count: 5 }, { count: 1 }, { count: 3 }];
    const sorted = result.current.sortItems(items, (item) => ({ count: item.count }));
    expect(sorted.map((i) => i.count)).toEqual([1, 3, 5]);
  });
});
