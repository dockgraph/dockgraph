// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from "vitest";
import { render, screen, fireEvent, cleanup } from "@testing-library/react";

afterEach(() => cleanup());

vi.mock("../theme", () => ({
  useTheme: () => ({
    theme: {
      panelBg: "#1e293b",
      panelBorder: "#334155",
      nodeText: "#e2e8f0",
      nodeSubtext: "#64748b",
    },
    toggle: vi.fn(),
  }),
}));

import { ViewTabs } from "./ViewTabs";

describe("ViewTabs", () => {
  it("renders all view tabs", () => {
    render(<ViewTabs activeView="graph" onViewChange={() => {}} />);
    expect(screen.getByText("Graph")).toBeDefined();
    expect(screen.getByText("Table")).toBeDefined();
    expect(screen.getByText("Dashboard")).toBeDefined();
  });

  it("marks the active tab with aria-current", () => {
    render(<ViewTabs activeView="graph" onViewChange={() => {}} />);
    expect(screen.getByText("Graph").getAttribute("aria-current")).toBe("page");
    expect(screen.getByText("Table").hasAttribute("aria-current")).toBe(false);
  });

  it("calls onViewChange when an inactive tab is clicked", () => {
    const onChange = vi.fn();
    render(<ViewTabs activeView="graph" onViewChange={onChange} />);
    fireEvent.click(screen.getByText("Table"));
    expect(onChange).toHaveBeenCalledWith("table");
  });

  it("does not call onViewChange when the active tab is clicked", () => {
    const onChange = vi.fn();
    render(<ViewTabs activeView="graph" onViewChange={onChange} />);
    fireEvent.click(screen.getByText("Graph"));
    expect(onChange).not.toHaveBeenCalled();
  });
});
