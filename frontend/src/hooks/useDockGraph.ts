import { useCallback, useEffect, useRef, useState } from 'react';
import type { DGNode, DGEdge, GraphSnapshot, WireMessage } from '../types';
import { RECONNECT_MAX_DELAY } from '../utils/constants';
import { snapshotFingerprint, applyDelta as applyDeltaFn } from './deltaUtils';

interface DockGraphState {
  nodes: DGNode[];
  edges: DGEdge[];
  connected: boolean;
  ready: boolean;
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
    ready: false,
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
    setState((prev) => ({ ...prev, nodes, edges, ready: true }));
  }, []);

  const applyDelta = useCallback((delta: Parameters<typeof applyDeltaFn>[1]) => {
    setState((prev) => ({ ...prev, ...applyDeltaFn(prev, delta), ready: true }));
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
      if (!msg || typeof msg.type !== 'string') return;
      if (typeof msg.version !== 'number') return;
      if (msg.type === 'auth_expired') {
        window.location.reload();
        return;
      }
      if (!('data' in msg) || !msg.data) return;
      if (msg.type === 'snapshot') {
        const d = msg.data;
        if (!Array.isArray(d.nodes) || !Array.isArray(d.edges)) return;
        applySnapshot(d);
      } else if (msg.type === 'delta') {
        applyDelta(msg.data);
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
