import { ReactFlowProvider } from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import { FlowCanvas } from './components/FlowCanvas';
import { useDockerFlow } from './hooks/useDockerFlow';
import { ThemeProvider, useTheme } from './theme';

function globalStyles(mode: 'dark' | 'light') {
  const isDark = mode === 'dark';
  const bg = isDark ? '#1e293b' : '#f7f5f1';
  const border = isDark ? '#334155' : '#d6d0c8';
  const text = isDark ? '#94a3b8' : '#5c5448';
  const hoverBg = isDark ? '#334155' : '#f3f0eb';

  return `
.react-flow__edges { z-index: 1000 !important; }

.react-flow__controls {
  background: ${bg} !important;
  border: 1px solid ${border} !important;
  border-radius: 6px !important;
  box-shadow: none !important;
}
.react-flow__controls button {
  background: ${bg} !important;
  border: none !important;
  border-bottom: 1px solid ${border} !important;
  color: ${text} !important;
  width: 28px !important;
  height: 28px !important;
}
.react-flow__controls button:last-child {
  border-bottom: none !important;
}
.react-flow__controls button:hover {
  background: ${hoverBg} !important;
}
.react-flow__controls button svg {
  fill: ${text} !important;
}
`;
}

function AppContent() {
  const { nodes, edges, connected } = useDockerFlow();
  const { theme } = useTheme();

  return (
    <div style={{ width: '100vw', height: '100vh' }}>
      <style>{globalStyles(theme.mode)}</style>
      <FlowCanvas dfNodes={nodes} dfEdges={edges} connected={connected} />
    </div>
  );
}

export default function App() {
  return (
    <ThemeProvider>
      <ReactFlowProvider>
        <AppContent />
      </ReactFlowProvider>
    </ThemeProvider>
  );
}
