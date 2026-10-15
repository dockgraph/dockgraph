// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from "vitest";
import { render, screen, fireEvent, cleanup } from "@testing-library/react";
import { ThemeProvider } from "../../theme";
import { ComposeProjectsCard } from "./ComposeProjectsCard";
import { STANDALONE_STACK } from "../../utils/stack";
import type { DGNode } from "../../types";

afterEach(() => cleanup());

const nodes: DGNode[] = [
  { id: "service:shop_web", type: "service", name: "shop_web", status: "running", stack: "shop" },
  { id: "service:shop_db", type: "service", name: "shop_db", status: "degraded", stack: "shop" },
  { id: "container:blog-app-1", type: "container", name: "blog-app-1", status: "running", labels: { "com.docker.compose.project": "blog" } },
  { id: "container:lone", type: "container", name: "lone", status: "exited" },
];

describe("ComposeProjectsCard", () => {
  it("counts services and containers per stack/project", () => {
    render(<ThemeProvider><ComposeProjectsCard nodes={nodes} /></ThemeProvider>);
    expect(screen.getByText("Compose Projects")).toBeDefined();
    expect(screen.getByText("shop")).toBeDefined();
    expect(screen.getByText("1/2")).toBeDefined();
    expect(screen.getByText("standalone")).toBeDefined();
  });

  it("is titled Stacks / Projects in swarm mode", () => {
    render(<ThemeProvider><ComposeProjectsCard nodes={nodes} swarm /></ThemeProvider>);
    expect(screen.getByText("Stacks / Projects")).toBeDefined();
  });

  it("selects a stack when a row is clicked", () => {
    const onSelectStack = vi.fn();
    render(<ThemeProvider><ComposeProjectsCard nodes={nodes} onSelectStack={onSelectStack} /></ThemeProvider>);
    fireEvent.click(screen.getByText("shop"));
    expect(onSelectStack).toHaveBeenCalledWith("shop");
    fireEvent.click(screen.getByText("standalone"));
    expect(onSelectStack).toHaveBeenCalledWith(STANDALONE_STACK);
  });
});
