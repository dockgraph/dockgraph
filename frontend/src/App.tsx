import { useMemo } from 'react';
import { ReactFlowProvider } from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import { ErrorBoundary } from './components/ErrorBoundary';
import { FlowCanvas } from './components/FlowCanvas';
import { useDockGraph } from './hooks/useDockGraph';
import { useContainerStats } from './hooks/useContainerStats';
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
  const { nodes, edges, connected, ready } = useDockGraph(handleStatsMessage);
  const { theme } = useTheme();
  const css = useMemo(() => globalStyles(theme), [theme]);

  return (
    <div style={{ width: '100vw', height: '100vh' }}>
      <style>{css}</style>
      <FlowCanvas dgNodes={nodes} dgEdges={edges} connected={connected} ready={ready} statsMap={stats} />
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
