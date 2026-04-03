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
      rowHover: "#263044",
    },
    toggle: vi.fn(),
  }),
}));

import { TableToolbar, type GroupOption } from "./TableToolbar";

const CONTAINER_OPTIONS: GroupOption[] = [
  { key: "compose", label: "Compose Project" },
  { key: "network", label: "Network" },
  { key: "status", label: "Status" },
  { key: "none", label: "None" },
];

const RESOURCE_OPTIONS: GroupOption[] = [
  { key: "compose", label: "Compose Project" },
  { key: "driver", label: "Driver" },
  { key: "none", label: "None" },
];

describe("TableToolbar", () => {
  it("renders group-by button with current value", () => {
    render(<TableToolbar groupBy="network" onGroupByChange={() => {}} options={CONTAINER_OPTIONS} />);
    expect(screen.getByText("Network")).toBeDefined();
  });

  it("calls onGroupByChange when an option is clicked", () => {
    const onChange = vi.fn();
    render(<TableToolbar groupBy="network" onGroupByChange={onChange} options={CONTAINER_OPTIONS} />);
    fireEvent.click(screen.getByText("Network"));
    fireEvent.click(screen.getByText("Status"));
    expect(onChange).toHaveBeenCalledWith("status");
  });

  it("shows all options when opened", () => {
    render(<TableToolbar groupBy="network" onGroupByChange={() => {}} options={CONTAINER_OPTIONS} />);
    fireEvent.click(screen.getByText("Network"));
    expect(screen.getByText("Compose Project")).toBeDefined();
    expect(screen.getByText("Status")).toBeDefined();
    expect(screen.getByText("None")).toBeDefined();
  });

  it("shows resource-specific options", () => {
    render(<TableToolbar groupBy="compose" onGroupByChange={() => {}} options={RESOURCE_OPTIONS} />);
    fireEvent.click(screen.getByText("Compose Project"));
    expect(screen.getByText("Driver")).toBeDefined();
    expect(screen.getByText("None")).toBeDefined();
  });
});
