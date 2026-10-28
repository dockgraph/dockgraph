// @vitest-environment jsdom
import { describe, it, expect, afterEach } from "vitest";
import { render, screen, cleanup } from "@testing-library/react";
import { ThemeProvider } from "../../theme";
import { DashboardCard } from "./DashboardCard";

afterEach(() => cleanup());

describe("DashboardCard", () => {
  it("names the node it describes under the title", () => {
    render(<ThemeProvider><DashboardCard title="Disk Usage" node="dgc-m1">body</DashboardCard></ThemeProvider>);
    const note = screen.getByText("dgc-m1");
    expect(note.closest("[title]")!.getAttribute("title")).toBe("Only the node running DockGraph, not the whole swarm");
    expect(screen.getByText("on")).toBeTruthy();
  });

  it("shows no node note without one", () => {
    render(<ThemeProvider><DashboardCard title="Disk Usage">body</DashboardCard></ThemeProvider>);
    expect(screen.queryByText("on")).toBeNull();
  });
});
