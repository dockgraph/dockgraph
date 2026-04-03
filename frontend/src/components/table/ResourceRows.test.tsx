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
      canvasBg: "#0f172a",
    },
    toggle: vi.fn(),
  }),
}));

import { NetworkRow } from "./NetworkRow";
import { VolumeRow } from "./VolumeRow";
import type { DGNode } from "../../types";

describe("NetworkRow", () => {
  const node: DGNode = { id: "network:app", type: "network", name: "app-network", driver: "bridge" };

  it("renders network name and driver", () => {
    render(<NetworkRow node={node} containerCount={2} selected={false} onClick={() => {}} gridTemplate="1fr 1fr 1fr 1fr 1fr" />);
    expect(screen.getByText("app-network")).toBeDefined();
    expect(screen.getByText("bridge")).toBeDefined();
  });

  it("renders container count", () => {
    render(<NetworkRow node={node} containerCount={2} selected={false} onClick={() => {}} gridTemplate="1fr 1fr 1fr 1fr 1fr" />);
    expect(screen.getByText("2")).toBeDefined();
  });

  it("calls onClick with node id", () => {
    const onClick = vi.fn();
    render(<NetworkRow node={node} containerCount={2} selected={false} onClick={onClick} gridTemplate="1fr 1fr 1fr 1fr 1fr" />);
    fireEvent.click(screen.getByText("app-network"));
    expect(onClick).toHaveBeenCalledWith("network:app");
  });
});

describe("VolumeRow", () => {
  const node: DGNode = { id: "volume:data", type: "volume", name: "pgdata", driver: "local" };

  it("renders volume name and driver", () => {
    render(<VolumeRow node={node} mountPath="/data" containerCount={1} selected={false} onClick={() => {}} gridTemplate="1fr 1fr 1fr 1fr" />);
    expect(screen.getByText("pgdata")).toBeDefined();
    expect(screen.getByText("local")).toBeDefined();
  });

  it("calls onClick with node id", () => {
    const onClick = vi.fn();
    render(<VolumeRow node={node} mountPath="/data" containerCount={1} selected={false} onClick={onClick} gridTemplate="1fr 1fr 1fr 1fr" />);
    fireEvent.click(screen.getByText("pgdata"));
    expect(onClick).toHaveBeenCalledWith("volume:data");
  });
});
