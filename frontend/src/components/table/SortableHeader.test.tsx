// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from "vitest";
import { render, screen, fireEvent, cleanup } from "@testing-library/react";

afterEach(() => cleanup());

vi.mock("../../theme", () => ({
  useTheme: () => ({
    theme: {
      canvasBg: "#0f172a",
      panelBorder: "#334155",
      nodeText: "#e2e8f0",
      nodeSubtext: "#64748b",
    },
    toggle: vi.fn(),
  }),
}));

import { SortableHeader } from "./SortableHeader";

describe("SortableHeader", () => {
  const columns = [
    { key: "name", label: "Name" },
    { key: "status", label: "Status" },
  ];

  it("renders all column labels", () => {
    render(
      <SortableHeader columns={columns} sortColumn="name" sortDirection="asc" onSort={() => {}} gridTemplate="1fr 1fr" />,
    );
    expect(screen.getByText("Name")).toBeDefined();
    expect(screen.getByText("Status")).toBeDefined();
  });

  it("shows ascending arrow for active sort column", () => {
    render(
      <SortableHeader columns={columns} sortColumn="name" sortDirection="asc" onSort={() => {}} gridTemplate="1fr 1fr" />,
    );
    const nameBtn = screen.getByText("Name").closest("button")!;
    expect(nameBtn.textContent).toContain("\u25B2");
  });

  it("shows descending arrow for active sort column", () => {
    render(
      <SortableHeader columns={columns} sortColumn="name" sortDirection="desc" onSort={() => {}} gridTemplate="1fr 1fr" />,
    );
    const nameBtn = screen.getByText("Name").closest("button")!;
    expect(nameBtn.textContent).toContain("\u25BC");
  });

  it("calls onSort with column key on click", () => {
    const onSort = vi.fn();
    render(
      <SortableHeader columns={columns} sortColumn="name" sortDirection="asc" onSort={onSort} gridTemplate="1fr 1fr" />,
    );
    fireEvent.click(screen.getByText("Status"));
    expect(onSort).toHaveBeenCalledWith("status");
  });
});
