import { useCallback, useEffect, useRef, useState } from 'react';
import type { DGNode, DGEdge, GraphSnapshot, DeltaUpdate, WireMessage } from '../types';
import { RECONNECT_MAX_DELAY } from '../utils/constants';

interface DockGraphState {
  nodes: DGNode[];
  edges: DGEdge[];
  connected: boolean;
}

/**
 * Produces a lightweight fingerprint of the current graph state.
 * Prevents redundant re-renders when the WebSocket pushes a snapshot
 * that is structurally identical to what we already have (same node IDs,
 * same statuses, same edge set).
 */
function snapshotFingerprint(nodes: DGNode[], edges: DGEdge[]): string {
  const nk = (nodes ?? []).map((n) => `${n.id}:${n.status ?? ''}`).join(',');
  const ek = (edges ?? []).map((e) => e.id).join(',');
  return nk + '|' + ek;
}

/**
 * Manages the WebSocket connection to the dockgraph backend.
 *
 * On mount, opens a WebSocket that receives the initial graph snapshot
 * followed by incremental delta updates as containers start/stop. The
 * connection automatically reconnects with exponential backoff (capped
 * at 30 seconds) if the server drops or the network blips.
 */
export function useDockGraph(): DockGraphState {
  const [state, setState] = useState<DockGraphState>({
    nodes: [],
    edges: [],
    connected: false,
  });

  const wsRef = useRef<WebSocket | null>(null);
  const retryRef = useRef(0);
  const retryTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const unmountedRef = useRef(false);
  const fingerprintRef = useRef('');
  const connectRef = useRef<(() => void) | null>(null);
  const maxRetryDelay = RECONNECT_MAX_DELAY;

  const applySnapshot = useCallback((snap: GraphSnapshot) => {
    const nodes = snap.nodes ?? [];
    const edges = snap.edges ?? [];
    const fp = snapshotFingerprint(nodes, edges);
    if (fp === fingerprintRef.current) return;
    fingerprintRef.current = fp;
    setState((prev) => ({ ...prev, nodes, edges }));
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
            nodes[idx] = { ...nodes[idx], ...update } as DGNode;
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
      let msg: WireMessage;
      try {
        msg = JSON.parse(event.data);
      } catch (e) {
        console.warn('failed to parse WebSocket message', e);
        return;
      }
      if (!msg || typeof msg.type !== 'string' || !msg.data) return;
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

      if (retryTimerRef.current) clearTimeout(retryTimerRef.current);
      const delay = Math.min(1000 * Math.pow(2, retryRef.current), maxRetryDelay);
      retryRef.current++;
      retryTimerRef.current = setTimeout(() => connectRef.current?.(), delay);
    };

    ws.onerror = () => {
      ws.close();
    };
  }, [applySnapshot, applyDelta, maxRetryDelay]);

  useEffect(() => {
    connectRef.current = connect;
  }, [connect]);

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
