// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { act, cleanup, renderHook } from '@testing-library/react';
import type { Node as RFNode } from '@xyflow/react';
import type { DGNode } from '../types';
import { toNodeGroupedFlowNodes } from '../utils/nodeTransform';

const fitView = vi.fn();
let canvas: RFNode[] = [];
vi.mock('@xyflow/react', () => ({
  useReactFlow: () => ({ fitView, getNodes: () => canvas }),
}));

import { useDetailPanel } from './useDetailPanel';

const mgr: DGNode = {
  id: 'swarmnode:mgr', type: 'swarmnode', name: 'mgr', status: 'ready',
  swarmNode: { id: 'n1', role: 'manager', leader: true, availability: 'active', state: 'ready' },
};
const wrk: DGNode = {
  id: 'swarmnode:wrk', type: 'swarmnode', name: 'wrk', status: 'ready',
  swarmNode: { id: 'n2', role: 'worker', availability: 'active', state: 'ready' },
};
const web: DGNode = {
  id: 'service:shop_web', type: 'service', name: 'shop_web', status: 'running', stack: 'shop',
  service: {
    mode: 'replicated',
    replicas: { running: 2, desired: 2 },
    tasks: [
      { id: 't1', slot: 1, nodeHostname: 'mgr', state: 'running', desiredState: 'running' },
      { id: 't2', slot: 2, nodeHostname: 'wrk', state: 'running', desiredState: 'running' },
    ],
  },
};
const graph = [mgr, wrk, web];

/** The ids passed to the last fitView call, after the panel's resize delay. */
function fittedIds(): string[] | undefined {
  act(() => { vi.advanceTimersByTime(100); });
  const call = fitView.mock.calls.at(-1);
  return call ? (call[0].nodes as { id: string }[]).map((n) => n.id) : undefined;
}

beforeEach(() => {
  vi.useFakeTimers();
  fitView.mockClear();
  // The service panel fetches its details; keep that request pending.
  vi.stubGlobal('fetch', vi.fn(() => new Promise(() => {})));
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe('useDetailPanel viewport focus', () => {
  it('centres on the clicked node in the network view', () => {
    canvas = [{ id: 'service:shop_web', position: { x: 0, y: 0 }, data: {} }];
    const { result } = renderHook(() => useDetailPanel(graph, []));
    act(() => result.current.handleInfoClick('service:shop_web'));
    expect(fittedIds()).toEqual(['service:shop_web']);
  });

  it('centres on the card a service was opened from in the per-node view', () => {
    canvas = toNodeGroupedFlowNodes(graph, null);
    const { result } = renderHook(() => useDetailPanel(graph, []));
    act(() => result.current.handleInfoClick('service:shop_web', 'nodesvc:wrk:shop_web'));
    expect(result.current.detailNodeId).toBe('service:shop_web');
    expect(fittedIds()).toEqual(['nodesvc:wrk:shop_web']);
  });

  it('centres on every card of a service opened without one', () => {
    canvas = toNodeGroupedFlowNodes(graph, null);
    const { result } = renderHook(() => useDetailPanel(graph, []));
    act(() => result.current.handleNavigate('service:shop_web'));
    expect(fittedIds()).toEqual(['nodesvc:mgr:shop_web', 'nodesvc:wrk:shop_web']);
  });

  it("centres on the service's card on the node whose panel linked to it", () => {
    canvas = toNodeGroupedFlowNodes(graph, null);
    const { result } = renderHook(() => useDetailPanel(graph, []));
    act(() => result.current.handleInfoClick('swarmnode:wrk'));
    act(() => result.current.handleNavigate('service:shop_web'));
    expect(result.current.detailNodeId).toBe('service:shop_web');
    expect(fittedIds()).toEqual(['nodesvc:wrk:shop_web']);
  });

  it('reports the canvas node every panel is opened for, panel links included', () => {
    canvas = toNodeGroupedFlowNodes(graph, null);
    const onOpen = vi.fn();
    const { result } = renderHook(() => useDetailPanel(graph, [], onOpen));
    act(() => result.current.handleInfoClick('swarmnode:wrk'));
    expect(onOpen).toHaveBeenLastCalledWith('swarmnode:wrk');
    // A link in that panel: the service, as seen on that node.
    act(() => result.current.handleNavigate('service:shop_web'));
    expect(onOpen).toHaveBeenLastCalledWith('nodesvc:wrk:shop_web');
  });

  it("opens a role group's panel from the graph data alone, centred on the group", () => {
    canvas = toNodeGroupedFlowNodes(graph, null);
    const fetchMock = vi.fn(() => new Promise(() => {}));
    vi.stubGlobal('fetch', fetchMock);
    const { result } = renderHook(() => useDetailPanel(graph, []));
    act(() => result.current.handleInfoClick('rolegroup:worker'));
    expect(result.current.variant).toEqual({ kind: 'rolegroup', role: 'worker' });
    expect(result.current.loading).toBe(false);
    expect(fetchMock).not.toHaveBeenCalled();
    expect(fittedIds()).toEqual(['rolegroup:worker']);
  });

  it("opens a node linked from its role group's panel, centred on that node", () => {
    canvas = toNodeGroupedFlowNodes(graph, null);
    const { result } = renderHook(() => useDetailPanel(graph, []));
    act(() => result.current.handleInfoClick('rolegroup:worker'));
    act(() => result.current.handleNavigate('swarmnode:wrk'));
    expect(result.current.variant).toEqual({ kind: 'swarmnode', node: wrk });
    expect(fittedIds()).toEqual(['swarmnode:wrk']);
  });

  it('opens nothing for an unknown role', () => {
    const { result } = renderHook(() => useDetailPanel(graph, []));
    act(() => result.current.handleInfoClick('rolegroup:observer'));
    expect(result.current.variant).toEqual({ kind: 'none' });
  });

  it('leaves the view alone for a resource the canvas does not show', () => {
    canvas = toNodeGroupedFlowNodes(graph, null);
    const { result } = renderHook(() => useDetailPanel(graph, []));
    act(() => result.current.handleInfoClick('network:shop_net'));
    expect(fittedIds()).toBeUndefined();
  });
});
