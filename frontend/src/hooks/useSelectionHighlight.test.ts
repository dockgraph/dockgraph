// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { act, cleanup, renderHook } from '@testing-library/react';
import type { Node as RFNode, Edge as RFEdge } from '@xyflow/react';
import { useSelectionHighlight } from './useSelectionHighlight';
import { FADE_OPACITY } from '../utils/constants';

vi.mock('@xyflow/react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@xyflow/react')>()),
  useStore: () => false, // not low zoom
}));

afterEach(cleanup);

const node = (id: string): RFNode => ({ id, type: 'containerNode', position: { x: 0, y: 0 }, data: {} });
const edge = (id: string, source: string, target: string): RFEdge => ({ id, source, target });
const faded = (nodes: RFNode[]) => nodes.filter((n) => n.style?.opacity === FADE_OPACITY).map((n) => n.id);

describe('useSelectionHighlight', () => {
  const stackA = { nodes: [node('a1'), node('a2'), node('a3')], edges: [edge('ea', 'a1', 'a2')] };
  const stackB = { nodes: [node('b1'), node('b2')], edges: [edge('eb', 'b1', 'b2')] };

  it('fades unrelated nodes around the selection', () => {
    const { result } = renderHook(() => useSelectionHighlight(stackA.nodes, stackA.edges));
    act(() => result.current.selectNode('a1'));
    expect(faded(result.current.styledNodes)).toEqual(['a3']);
  });

  it('ignores a selected node missing from a new topology', () => {
    const { result, rerender } = renderHook((g) => useSelectionHighlight(g.nodes, g.edges), { initialProps: stackA });
    act(() => result.current.selectNode('a1'));
    rerender(stackB);
    expect(faded(result.current.styledNodes)).toEqual([]);
    expect(result.current.styledNodes).toBe(stackB.nodes);
  });

  it('ignores a selected edge missing from a new topology', () => {
    const { result, rerender } = renderHook((g) => useSelectionHighlight(g.nodes, g.edges), { initialProps: stackA });
    act(() => result.current.selectEdge('ea'));
    rerender(stackB);
    expect(faded(result.current.styledNodes)).toEqual([]);
  });

  it('highlights the selection again when its node comes back', () => {
    const { result, rerender } = renderHook((g) => useSelectionHighlight(g.nodes, g.edges), { initialProps: stackA });
    act(() => result.current.selectNode('a1'));
    rerender(stackB);
    rerender(stackA);
    expect(faded(result.current.styledNodes)).toEqual(['a3']);
  });

  it('keeps a selected link edge that only exists in the selection links', () => {
    const link = edge('link', 'a1', 'a3');
    const linkEdgesFor = () => [link];
    const { result } = renderHook(() => useSelectionHighlight(stackA.nodes, stackA.edges, false, null, linkEdgesFor));
    act(() => result.current.selectEdge('link'));
    expect(faded(result.current.styledNodes)).toEqual(['a2']);
  });
});
