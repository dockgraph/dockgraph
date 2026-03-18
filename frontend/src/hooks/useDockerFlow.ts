import { useCallback, useEffect, useRef, useState } from 'react';
import type { DFNode, DFEdge, GraphSnapshot, DeltaUpdate, WireMessage } from '../types';

interface DockerFlowState {
  nodes: DFNode[];
  edges: DFEdge[];
  connected: boolean;
}

export function useDockerFlow(): DockerFlowState {
  const [state, setState] = useState<DockerFlowState>({
    nodes: [],
    edges: [],
    connected: false,
  });

  const wsRef = useRef<WebSocket | null>(null);
  const retryRef = useRef(0);
  const retryTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const unmountedRef = useRef(false);
  const maxRetryDelay = 30_000;

  const applySnapshot = useCallback((snap: GraphSnapshot) => {
    setState((prev) => {
      if (
        JSON.stringify(prev.nodes) === JSON.stringify(snap.nodes) &&
        JSON.stringify(prev.edges) === JSON.stringify(snap.edges)
      ) {
        return prev;
      }
      return { ...prev, nodes: snap.nodes, edges: snap.edges };
    });
  }, []);

  const applyDelta = useCallback((delta: DeltaUpdate) => {
    setState((prev) => {
      let nodes = [...prev.nodes];
      let edges = [...prev.edges];

      if (delta.nodesRemoved) {
        const removed = new Set(delta.nodesRemoved);
        nodes = nodes.filter((n) => !removed.has(n.id));
      }

      if (delta.nodesAdded) {
        nodes = [...nodes, ...delta.nodesAdded];
      }

      if (delta.nodesUpdated) {
        for (const update of delta.nodesUpdated) {
          const idx = nodes.findIndex((n) => n.id === update.id);
          if (idx !== -1) {
            nodes[idx] = { ...nodes[idx], ...update } as DFNode;
          }
        }
      }

      if (delta.edgesRemoved) {
        const removed = new Set(delta.edgesRemoved);
        edges = edges.filter((e) => !removed.has(e.id));
      }

      if (delta.edgesAdded) {
        edges = [...edges, ...delta.edgesAdded];
      }

      return { ...prev, nodes, edges };
    });
  }, []);

  const connect = useCallback(() => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const url = `${protocol}//${window.location.host}/ws`;

    const ws = new WebSocket(url);
    wsRef.current = ws;

    ws.onopen = () => {
      retryRef.current = 0;
      setState((prev) => ({ ...prev, connected: true }));
    };

    ws.onmessage = (event) => {
      const msg: WireMessage = JSON.parse(event.data);
      if (msg.type === 'snapshot') {
        applySnapshot(msg.data as GraphSnapshot);
      } else if (msg.type === 'delta') {
        applyDelta(msg.data as DeltaUpdate);
      }
    };

    ws.onclose = () => {
      setState((prev) => ({ ...prev, connected: false }));
      wsRef.current = null;

      if (unmountedRef.current) return;

      const delay = Math.min(1000 * Math.pow(2, retryRef.current), maxRetryDelay);
      retryRef.current++;
      retryTimerRef.current = setTimeout(connect, delay);
    };

    ws.onerror = () => {
      ws.close();
    };
  }, [applySnapshot, applyDelta]);

  useEffect(() => {
    unmountedRef.current = false;
    connect();
    return () => {
      unmountedRef.current = true;
      if (retryTimerRef.current) clearTimeout(retryTimerRef.current);
      wsRef.current?.close();
    };
  }, [connect]);

  return state;
}
