# DockGraph — Frontend

React application that renders real-time Docker infrastructure as an interactive graph and table view. Receives topology and stats data over WebSocket and visualizes containers, networks, and volumes using the [ELK](https://www.eclipse.org/elk/) layout algorithm for graph mode and sortable, groupable tables for list mode.

## Tech Stack

- **React 19** with TypeScript
- **[React Flow](https://reactflow.dev/)** (`@xyflow/react`) — graph rendering, panning, zooming, minimap
- **[ELK.js](https://github.com/kieler/elkjs)** — automatic hierarchical graph layout
- **Vite** — dev server with HMR, production bundling
- **Vitest** — unit testing with jsdom environment

## Project Structure

```
src/
├── components/
│   ├── FlowCanvas.tsx           # Main canvas — orchestrates views, layout, and detail panels
│   ├── ContainerNode.tsx        # Container node with status, image, ports
│   ├── NetworkGroup.tsx         # Network group (parent node for containers)
│   ├── VolumeNode.tsx           # Volume node
│   ├── ElkEdge.tsx              # Custom edge with animated flow dots
│   ├── CanvasEdgeLayer.tsx      # Canvas-based edge renderer for large graphs
│   ├── ViewTabs.tsx             # Graph / Table / Dashboard navigation tabs
│   ├── SearchFilter.tsx         # Search input with type/status filter chips
│   ├── SearchFilterChips.tsx    # Filter chip buttons (type, status)
│   ├── InspectButton.tsx        # Node inspect action button
│   ├── StatsMini.tsx            # Compact CPU/memory stats overlay
│   ├── StatusIndicator.tsx      # WebSocket connection status badge
│   ├── ThemeToggle.tsx          # Dark/light theme switch
│   ├── LogoutButton.tsx         # Session logout (when auth enabled)
│   ├── panels/                  # Detail panel components
│   │   ├── DetailPanel.tsx          # Panel container with slide-in animation
│   │   ├── DetailPanelHeader.tsx    # Container header with status and image
│   │   ├── DetailPanelStats.tsx     # Real-time CPU, memory, network, block I/O
│   │   ├── DetailPanelProcess.tsx   # Process info (command, entrypoint, user)
│   │   ├── DetailPanelPorts.tsx     # Port mappings
│   │   ├── DetailPanelMounts.tsx    # Volume and bind mounts
│   │   ├── DetailPanelEnv.tsx       # Environment variables
│   │   ├── DetailPanelLabels.tsx    # Container labels
│   │   ├── DetailPanelNetwork.tsx   # Network interfaces and IPs
│   │   ├── DetailPanelNetworkInfo.tsx # Network detail (IPAM, containers)
│   │   ├── DetailPanelSecurity.tsx  # Security options (privileged, caps)
│   │   ├── DetailPanelHealth.tsx    # Health check configuration and status
│   │   ├── DetailPanelLogs.tsx      # Live log streaming
│   │   ├── DetailPanelCompose.tsx   # Compose service configuration
│   │   ├── DetailPanelVolume.tsx    # Volume detail (driver, usage, mounts)
│   │   ├── GhostVolumePanel.tsx     # Ghost (not-running) volume detail
│   │   ├── GhostNetworkPanel.tsx    # Ghost network detail
│   │   ├── GhostContainerPanel.tsx  # Ghost container detail
│   │   ├── panelStyles.ts          # Shared panel style helpers
│   │   └── shared.tsx              # Section, Row, and style re-exports
│   └── table/                   # Table view components
│       ├── TableView.tsx            # Resource tabs (Containers/Networks/Volumes)
│       ├── GroupedTable.tsx         # Generic grouped table with toolbar and cards
│       ├── ContainerTable.tsx       # Container table with stats sub-rows
│       ├── ContainerRow.tsx         # Container row with network colors
│       ├── NetworkTable.tsx         # Network table with container counts
│       ├── NetworkRow.tsx           # Network row with subnet/gateway
│       ├── VolumeTable.tsx          # Volume table with mount paths
│       ├── VolumeRow.tsx            # Volume row with RTL path truncation
│       ├── SortableHeader.tsx       # Clickable sort column headers
│       ├── TableGroupHeader.tsx     # Collapsible group header with chevron
│       ├── TableToolbar.tsx         # Group-by dropdown toolbar
│       └── tableStyles.ts          # Shared table style constants
├── hooks/
│   ├── useDockGraph.ts             # WebSocket connection, reconnect, delta updates
│   ├── useGraphLayout.ts           # ELK layout orchestration
│   ├── useSelectionHighlight.ts    # Click-to-highlight with opacity fading
│   ├── useSearchFilter.ts          # Text + type + status filtering
│   ├── useContainerDetail.ts       # Container inspect API integration
│   ├── useContainerStats.ts        # Real-time container stats via WebSocket
│   ├── useContainerLogs.ts         # Live log streaming
│   ├── useNetworkDetail.ts         # Network inspect API integration
│   ├── useVolumeDetail.ts          # Volume inspect API integration
│   ├── useResourceDetail.ts        # Shared detail fetching logic
│   ├── useTableSort.ts             # Generic column sort with toggle
│   ├── useTableGrouping.ts         # Group by compose/network/status/driver
│   ├── useRowHover.ts              # Shared row hover/selected state
│   └── useLogs.ts                  # Log stream management
├── layout/                      # ELK layout pipeline
│   ├── elk.ts                      # Pipeline orchestrator
│   ├── elkGraph.ts                 # Build ELK hierarchy from React Flow nodes
│   ├── elkPositions.ts             # Map ELK positions back to React Flow
│   ├── edgePaths.ts                # Extract and smooth SVG edge paths
│   └── components.ts               # Connected component detection (DFS)
├── canvas/                      # Canvas edge rendering
│   ├── canvasRenderer.ts           # Level-of-detail canvas edge painter
│   ├── canvasEdgeUtils.ts          # Edge path math utilities
│   └── canvasEdgeTypes.ts          # Canvas edge type definitions
├── utils/
│   ├── graphTransform.ts           # Domain model → React Flow conversion
│   ├── colors.ts                   # Deterministic network colors, status palette
│   ├── constants.ts                # Shared layout and visual constants
│   ├── formatBytes.ts              # Byte formatting utility
│   ├── logParser.ts                # Docker log stream parser
│   ├── nodeStyles.ts               # Node dimension calculations
│   └── pathUtils.ts                # SVG path utilities
├── types/
│   └── stats.ts                # Stats, detail, and log types
├── theme.tsx                    # Dark/light theme context with localStorage
├── types.ts                     # Domain types (DGNode, DGEdge, GraphSnapshot)
├── App.tsx                      # Root component with providers and global styles
└── main.tsx                     # Entry point
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

# Run tests
npx vitest run

# Production build
npm run build
```

The dev server starts on `http://localhost:5173` and proxies WebSocket and API requests to the Go backend at `localhost:7800`. Make sure the backend is running first.

## Architecture

### Data Flow

```
Backend (WebSocket) → useDockGraph hook → FlowCanvas
                                              ↓
                              ┌───────────────┴───────────────┐
                              │                               │
                        Graph View                      Table View
                              │                               │
                  graphTransform → ELK layout      useTableSort + useTableGrouping
                              │                               │
                  React Flow renders                 GroupedTable renders
                  nodes + edges                      sorted/grouped rows
                              |                               |
                              └───────────┬───────────────────┘
                                          │
                              Detail Panels (shared)
                              useContainerDetail / useVolumeDetail / useNetworkDetail
```

1. **`useDockGraph`** connects to `/ws`, receives an initial snapshot, then incremental delta updates as containers start/stop. Auto-reconnects with exponential backoff.

2. **Graph view**: `graphTransform` converts the backend's flat node/edge model into React Flow's hierarchical structure — containers are grouped inside network groups, volumes are placed in the group of their first consumer. The ELK layout pipeline then computes positions.

3. **Table view**: `TableView` splits nodes by type into resource tabs. Each table uses `useTableGrouping` (compose project, network, status, or driver) and `useTableSort` for interactive column sorting. `GroupedTable` provides the shared rendering pattern.

4. **Detail panels**: Clicking any resource in either view opens a detail panel. Container panels fetch live stats, logs, and inspect data. Network and volume panels show IPAM configuration and mount relationships.

### Key Design Decisions

- **ELK over dagre/d3-force**: ELK handles hierarchical grouping (containers inside network groups) natively and produces clean orthogonal edge routing. Force-directed layouts don't respect parent-child containment.

- **Canvas-based edge rendering**: For large graphs, edges are rendered on a `<canvas>` element with level-of-detail optimization instead of individual SVG elements.

- **Shared GroupedTable**: All three resource tables compose a single `GroupedTable` component, avoiding duplication of toolbar, group cards, sort headers, and empty states.

- **Delta updates**: After the initial snapshot, the backend sends only changed nodes/edges. The `useDockGraph` hook merges deltas into the current state to minimize re-renders.

- **Theme tokens**: All colors reference theme context values — no hardcoded colors. Table-specific tokens (`rowHover`, `cardBg`, `statsRowBg`) were added to avoid magic values.
