import { ReactFlowProvider } from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import { FlowCanvas } from './components/FlowCanvas';
import { useDockerFlow } from './hooks/useDockerFlow';

function AppContent() {
  const { nodes, edges, connected } = useDockerFlow();

  return (
    <div style={{ width: '100vw', height: '100vh' }}>
      <FlowCanvas dfNodes={nodes} dfEdges={edges} connected={connected} />
    </div>
  );
}

export default function App() {
  return (
    <ReactFlowProvider>
      <AppContent />
    </ReactFlowProvider>
  );
}
