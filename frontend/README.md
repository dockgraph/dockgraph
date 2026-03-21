# Docker Flow — Frontend

React application that renders a real-time, interactive graph of your Docker infrastructure. Receives topology data over WebSocket and visualizes containers, networks, and volumes as a hierarchical graph using the [ELK](https://www.eclipse.org/elk/) layout algorithm.

## Tech Stack

- **React 19** with TypeScript
- **[React Flow](https://reactflow.dev/)** (`@xyflow/react`) — graph rendering, panning, zooming, minimap
- **[ELK.js](https://github.com/kieler/elkjs)** — automatic hierarchical graph layout
- **Vite** — dev server with HMR, production bundling

## Project Structure

```
src/
├── components/          # React Flow node and edge renderers
│   ├── FlowCanvas.tsx       # Main canvas — orchestrates layout and rendering
│   ├── ContainerNode.tsx     # Container node with status, image, ports
│   ├── NetworkGroup.tsx      # Network group (parent node for containers)
│   ├── VolumeNode.tsx        # Volume node
│   ├── ElkEdge.tsx           # Custom edge with animated flow dots
│   ├── StatusIndicator.tsx   # WebSocket connection status badge
│   └── ThemeToggle.tsx       # Dark/light theme switch
├── hooks/
│   ├── useDockerFlow.ts      # WebSocket connection, reconnect, delta updates
│   └── useSelectionHighlight.ts  # Click-to-highlight with opacity fading
├── layout/              # ELK layout pipeline
│   ├── elk.ts               # Pipeline orchestrator
│   ├── elkGraph.ts           # Build ELK hierarchy from React Flow nodes
│   ├── elkPositions.ts       # Map ELK positions back to React Flow
│   ├── edgePaths.ts          # Extract and smooth SVG edge paths
│   └── components.ts         # Connected component detection (DFS)
├── utils/
│   ├── graphTransform.ts     # Domain model → React Flow conversion
│   ├── colors.ts             # Deterministic network colors, status palette
│   └── constants.ts          # Shared layout and visual constants
├── theme.tsx            # Dark/light theme context with localStorage
├── types.ts             # Domain types (DFNode, DFEdge, GraphSnapshot)
├── App.tsx              # Root component with providers and global styles
└── main.tsx             # Entry point
```

## Development

```bash
# Install dependencies
npm install

# Start dev server (proxies /ws and /healthz to backend on :7800)
npm run dev

# Type check
npx tsc --noEmit

# Lint
npm run lint

# Production build
npm run build
```

The dev server starts on `http://localhost:5173` and proxies WebSocket and API requests to the Go backend at `localhost:7800`. Make sure the backend is running first.

## Architecture

### Data Flow

```
Backend (WebSocket) → useDockerFlow hook → FlowCanvas
                                              ↓
                              graphTransform (domain → React Flow)
                                              ↓
                              ELK layout pipeline (auto-positioning)
                                              ↓
                              React Flow renders nodes + edges
```

1. **`useDockerFlow`** connects to `/ws`, receives an initial snapshot, then incremental delta updates as containers start/stop. Auto-reconnects with exponential backoff.

2. **`graphTransform`** converts the backend's flat node/edge model into React Flow's hierarchical structure — containers are grouped inside network groups, volumes are placed in the group of their first consumer.

3. **ELK layout pipeline** runs the Eclipse Layout Kernel to compute positions. The pipeline classifies nodes, builds the ELK hierarchy, runs the algorithm, maps positions back, and smooths edge paths.

4. **React Flow** renders the positioned graph with custom node components, animated edges, minimap, and controls.

### Key Design Decisions

- **ELK over dagre/d3-force**: ELK handles hierarchical grouping (containers inside network groups) natively and produces clean orthogonal edge routing. Force-directed layouts don't respect parent-child containment.

- **Canvas-based label measurement**: Node widths are computed by measuring label text with an off-screen canvas before layout, so ELK can produce a balanced graph without content overflow.

- **Hierarchy depth consistency**: Volumes are assigned to the network group of their first consumer. This ensures all edge endpoints are at the same hierarchy depth, which ELK requires for correct cross-group edge routing.

- **Delta updates**: After the initial snapshot, the backend sends only changed nodes/edges. The `useDockerFlow` hook merges deltas into the current state to minimize re-renders.
