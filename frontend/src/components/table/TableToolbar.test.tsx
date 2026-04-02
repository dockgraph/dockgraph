// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from "vitest";
import { render, screen, fireEvent, cleanup } from "@testing-library/react";

afterEach(() => cleanup());

vi.mock("../../theme", () => ({
  useTheme: () => ({
    theme: {
      mode: "dark",
      panelText: "#94a3b8",
      panelBg: "#1e293b",
      panelBorder: "#334155",
      nodeText: "#e2e8f0",
      nodeSubtext: "#64748b",
      canvasBg: "#0f172a",
    },
    toggle: vi.fn(),
  }),
}));

import { TableToolbar } from "./TableToolbar";

describe("TableToolbar", () => {
  it("renders group-by button with current value", () => {
    render(<TableToolbar groupBy="network" onGroupByChange={() => {}} />);
    expect(screen.getByText("Network")).toBeDefined();
  });

  it("calls onGroupByChange when an option is clicked", () => {
    const onChange = vi.fn();
    render(<TableToolbar groupBy="network" onGroupByChange={onChange} />);
    // Open the dropdown
    fireEvent.click(screen.getByText("Network"));
    // Click an option
    fireEvent.click(screen.getByText("Status"));
    expect(onChange).toHaveBeenCalledWith("status");
  });

  it("shows all options when opened", () => {
    render(<TableToolbar groupBy="network" onGroupByChange={() => {}} />);
    fireEvent.click(screen.getByText("Network"));
    expect(screen.getByText("Compose Project")).toBeDefined();
    expect(screen.getByText("Status")).toBeDefined();
    expect(screen.getByText("None")).toBeDefined();
  });
});
