import { ReactFlowProvider } from '@xyflow/react';
import '@xyflow/react/dist/style.css';

export default function App() {
  return (
    <ReactFlowProvider>
      <div style={{ width: '100vw', height: '100vh', background: '#0f172a' }}>
        <p style={{ color: '#94a3b8', padding: 20 }}>Docker Flow — connecting...</p>
      </div>
    </ReactFlowProvider>
  );
}
