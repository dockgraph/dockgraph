import { useMemo } from 'react';
import { ReactFlowProvider } from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import { ErrorBoundary } from './components/ErrorBoundary';
import { FlowCanvas } from './components/FlowCanvas';
import { useDockGraph } from './hooks/useDockGraph';
import { useContainerStats } from './hooks/useContainerStats';
import { useStackScope } from './hooks/useStackScope';
import { filterGraphByStack, isNodeStatsKey, isWorkload, listStacks, serviceOfTask } from './utils/stack';
import type { ContainerStatsData } from './types';
import { ThemeProvider, useTheme, type Theme } from './theme';

function globalStyles(theme: Theme) {
  const { panelBg: bg, panelBorder: border, panelText: text, rowHover: hoverBg, accent } = theme;

  return `
*, *::before, *::after { box-sizing: border-box; }

html, body, #root { margin: 0; height: 100%; }
body {
  font-family: var(--dg-font-ui);
  background: ${theme.canvasBg};
  color: ${theme.nodeText};
  -webkit-font-smoothing: antialiased;
  text-rendering: optimizeLegibility;
}

.dg-search-input::placeholder { color: ${theme.nodeSubtext}; opacity: 1; }

.dg-resource-tab:not(.dg-resource-tab--active):hover { color: ${theme.nodeText} !important; }

.dg-panel-close {
  display: grid; place-items: center;
  width: 28px; height: 28px; padding: 0;
  background: transparent; border: 1px solid transparent; border-radius: 7px;
  color: ${text}; font-size: 15px; line-height: 1; cursor: pointer;
  transition: background 0.15s, color 0.15s, border-color 0.15s;
}
.dg-panel-close:hover { background: ${hoverBg}; border-color: ${border}; color: ${accent}; }

.dg-copy { cursor: copy; border-radius: 3px; }
.dg-copy:hover { background: ${hoverBg}; box-shadow: 0 0 0 2px ${hoverBg}; }

.dg-iconbtn {
  display: grid; place-items: center;
  width: 30px; height: 30px; padding: 0;
  background: ${theme.canvasBg}; border: 1px solid ${border}; border-radius: 8px;
  color: ${text}; cursor: pointer;
  transition: background 0.15s, color 0.15s, border-color 0.15s;
}
.dg-iconbtn:hover { background: ${hoverBg}; color: ${accent}; }

@keyframes dg-spin { to { transform: rotate(360deg); } }
.dg-spinner {
  width: 18px; height: 18px; border-radius: 50%;
  border: 2px solid ${border}; border-top-color: ${accent};
  animation: dg-spin 0.7s linear infinite;
}
@media (prefers-reduced-motion: reduce) { .dg-spinner { animation-duration: 1.6s; } }

@keyframes dg-pulse {
  0%   { box-shadow: 0 0 6px var(--dg-glow), 0 0 0 0 var(--dg-pulse); }
  70%  { box-shadow: 0 0 6px var(--dg-glow), 0 0 0 6px transparent; }
  100% { box-shadow: 0 0 6px var(--dg-glow), 0 0 0 0 transparent; }
}
.dg-live-dot { animation: dg-pulse 2.4s ease-out infinite; }
@media (prefers-reduced-motion: reduce) {
  .dg-live-dot { animation: none; }
}

.react-flow__edges { z-index: 1000 !important; }
.react-flow__edge path { shape-rendering: optimizeSpeed; }
.react-flow__node { contain: layout style paint; }
/* Network groups carry a legend that straddles the top border, so they must be
   allowed to paint outside their box (container nodes keep full containment). */
.react-flow__node-networkGroup { contain: layout style !important; overflow: visible !important; }
/* The control-plane badge centers itself on its anchor via translate(-50%, -50%),
   so its content always extends outside its own (auto-sized, near-zero) box.
   It also sits right where the control spokes cross the gap, so it is lifted
   above the edge layer (z-index 1000 above) to keep the spokes behind it. */
.react-flow__node-controlSummary { contain: layout style !important; overflow: visible !important; z-index: 1001 !important; }

.react-flow__controls {
  background: ${bg} !important;
  border: 1px solid ${border} !important;
  border-radius: 8px !important;
  box-shadow: 0 2px 10px -4px rgba(0, 0, 0, 0.5) !important;
  overflow: hidden !important;
  margin: 0 !important;
  padding: 0 !important;
}
.react-flow__controls button {
  background: ${bg} !important;
  border: none !important;
  border-bottom: 1px solid ${border} !important;
  color: ${text} !important;
  width: 30px !important;
  height: 30px !important;
  transition: background 0.15s !important;
}
.react-flow__controls button:last-child {
  border-bottom: none !important;
}
.react-flow__controls button:hover {
  background: ${hoverBg} !important;
}
.react-flow__controls button svg {
  fill: ${text} !important;
  transition: fill 0.15s !important;
}
.react-flow__controls button:hover svg {
  fill: ${accent} !important;
}

.dg-theme-toggle {
  width: 30px;
  height: 30px;
  background: ${bg};
  border: 1px solid ${border};
  border-radius: 8px;
  box-shadow: 0 2px 10px -4px rgba(0, 0, 0, 0.5);
  color: ${text};
  line-height: 1;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  transition: background 0.15s, color 0.15s;
}
.dg-theme-toggle:hover {
  background: ${hoverBg};
  color: ${accent};
}

.react-flow__attribution {
  background: ${theme.canvasBg} !important;
  padding-right: 15px !important;
}
.react-flow__attribution a {
  color: ${text} !important;
}
`;
}

function AppContent() {
  const { stats, handleStatsMessage } = useContainerStats();
  const { nodes: allNodes, edges: allEdges, connected, ready } = useDockGraph(handleStatsMessage);
  const { theme } = useTheme();
  const css = useMemo(() => globalStyles(theme), [theme]);

  // Scope everything downstream (graph layout, search, table, dashboard, logs)
  // to the selected stack. Filtering upstream changes the topology key, so the
  // ELK layout reruns for the scoped graph on its own.
  const { selectedStack, setSelectedStack } = useStackScope();
  const stacks = useMemo(() => listStacks(allNodes), [allNodes]);
  const { nodes, edges } = useMemo(
    () => filterGraphByStack(allNodes, allEdges, selectedStack),
    [allNodes, allEdges, selectedStack],
  );
  // Only the scoped workloads' stats (keyed by container/service name, plus
  // per-task entries `{service}.{slot}.{taskId}` from node agents). The
  // per-swarm-node aggregates (`node:{hostname}`) belong to no stack and are
  // passed through untouched: swarm nodes stay visible in every scope, and
  // consumer lists (top consumers, alerts) drop them themselves.
  const scopedStats = useMemo(() => {
    if (!selectedStack) return stats;
    const names = new Set(nodes.filter(isWorkload).map((n) => n.name));
    const scoped = new Map<string, ContainerStatsData>();
    for (const [key, value] of stats) {
      if (isNodeStatsKey(key) || names.has(key) || names.has(serviceOfTask(key) ?? '')) scoped.set(key, value);
    }
    return scoped;
  }, [stats, nodes, selectedStack]);

  return (
    <div style={{ width: '100vw', height: '100vh' }}>
      <style>{css}</style>
      <FlowCanvas
        dgNodes={nodes}
        dgEdges={edges}
        connected={connected}
        ready={ready}
        statsMap={scopedStats}
        stacks={stacks}
        selectedStack={selectedStack}
        onSelectStack={setSelectedStack}
      />
    </div>
  );
}

export default function App() {
  return (
    <ThemeProvider>
      <ReactFlowProvider>
        <ErrorBoundary>
          <AppContent />
        </ErrorBoundary>
      </ReactFlowProvider>
    </ThemeProvider>
  );
}
