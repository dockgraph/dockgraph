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
    },
    toggle: vi.fn(),
  }),
}));

import { TableGroupHeader } from "./TableGroupHeader";

describe("TableGroupHeader", () => {
  it("renders label and count", () => {
    render(<TableGroupHeader label="frontend" count={3} collapsed={false} onToggle={() => {}} />);
    expect(screen.getByText("frontend")).toBeDefined();
    expect(screen.getByText("(3)")).toBeDefined();
  });

  it("calls onToggle when clicked", () => {
    const onToggle = vi.fn();
    render(<TableGroupHeader label="frontend" count={3} collapsed={false} onToggle={onToggle} />);
    fireEvent.click(screen.getByText("frontend"));
    expect(onToggle).toHaveBeenCalled();
  });

  it("renders collapsed chevron when collapsed", () => {
    const { container } = render(
      <TableGroupHeader label="frontend" count={3} collapsed={true} onToggle={() => {}} />,
    );
    const chevron = container.querySelector("[data-testid='chevron']") as HTMLElement;
    expect(chevron.style.transform).toBe("rotate(0deg)");
  });
});
