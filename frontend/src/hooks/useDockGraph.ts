import { useCallback, useEffect, useRef, useState } from 'react';
import type { DGNode, DGEdge, GraphSnapshot, WireMessage } from '../types';
import type { StatsMessage } from '../types/stats';
import { RECONNECT_MAX_DELAY, WS_IDLE_TIMEOUT } from '../utils/constants';
import { snapshotFingerprint, applyDelta as applyDeltaFn } from '../utils/deltaUtils';
import { protocolAction } from '../utils/protocol';

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
 * at 30 seconds) if the server drops or the network blips. A connection
 * that stays silent for WS_IDLE_TIMEOUT (no heartbeat) is dropped and
 * replaced the same way: through the swarm routing mesh, the new one
 * reaches a live server replica.
 */
export function useDockGraph(onStats?: (data: StatsMessage) => void): DockGraphState {
  const [state, setState] = useState<DockGraphState>({
    nodes: [],
    edges: [],
    connected: false,
    ready: false,
  });

  const wsRef = useRef<WebSocket | null>(null);
  const retryRef = useRef(0);
  const retryTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const idleTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const unmountedRef = useRef(false);
  const fingerprintRef = useRef('');
  const connectRef = useRef<(() => void) | null>(null);
  const onStatsRef = useRef(onStats);
  useEffect(() => { onStatsRef.current = onStats; }, [onStats]);
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

    const clearIdleTimer = () => {
      if (idleTimerRef.current) clearTimeout(idleTimerRef.current);
      idleTimerRef.current = null;
    };

    const disconnected = () => {
      // A socket already replaced (e.g. closed by a remount) must leave the
      // current one, its idle timer and the retry schedule alone.
      if (wsRef.current !== ws) return;
      clearIdleTimer();
      wsRef.current = null;
      setState((prev) => ({ ...prev, connected: false }));

      if (unmountedRef.current) return;

      if (retryTimerRef.current) clearTimeout(retryTimerRef.current);
      const delay = Math.min(1000 * Math.pow(2, retryRef.current), maxRetryDelay);
      retryRef.current++;
      retryTimerRef.current = setTimeout(() => connectRef.current?.(), delay);
    };

    // Restarted by every message, including heartbeats. When it fires the
    // server is gone without closing the socket, so the closing handshake
    // would never complete: detach the socket and reconnect right away.
    const armIdleTimer = () => {
      if (wsRef.current !== ws) return;
      clearIdleTimer();
      idleTimerRef.current = setTimeout(() => {
        ws.onopen = null;
        ws.onmessage = null;
        ws.onclose = null;
        ws.onerror = null;
        ws.close();
        disconnected();
      }, WS_IDLE_TIMEOUT);
    };
    armIdleTimer();

    ws.onopen = () => {
      retryRef.current = 0;
      armIdleTimer();
      setState((prev) => ({ ...prev, connected: true }));
    };

    ws.onmessage = (event) => {
      armIdleTimer();
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
      // A different protocol version means this bundle and the server
      // disagree on the payload shape: reload once to get a matching bundle.
      const action = protocolAction(msg.version);
      if (action === 'reload') {
        window.location.reload();
        return;
      }
      if (action === 'ignore') {
        console.warn(`unsupported protocol version ${msg.version}`);
        return;
      }
      if (!('data' in msg) || !msg.data) return;
      if (msg.type === 'snapshot') {
        const d = msg.data;
        if (!Array.isArray(d.nodes) || !Array.isArray(d.edges)) return;
        applySnapshot(d);
      } else if (msg.type === 'delta') {
        applyDelta(msg.data);
      } else if (msg.type === 'stats') {
        onStatsRef.current?.(msg.data as StatsMessage);
      }
    };

    ws.onclose = disconnected;

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
      if (idleTimerRef.current) clearTimeout(idleTimerRef.current);
      wsRef.current?.close();
    };
  }, [connect]);

  return state;
}
