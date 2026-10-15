# DockGraph — Frontend

React application that renders real-time Docker infrastructure as an interactive graph and table view. Receives topology and stats data over WebSocket and visualizes containers, networks, and volumes using the [ELK](https://www.eclipse.org/elk/) layout algorithm for graph mode and sortable, groupable tables for list mode. In swarm mode it also shows swarm services, their tasks and the cluster's nodes, with a per-node graph layout and a stack selector that scopes every view.

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
│   ├── ServiceNode.tsx          # Swarm service node with mode chip and replica badge
│   ├── SwarmNodeGroup.tsx       # Node view: one swarm node box with CPU/memory bars and service cards
│   ├── NodeServiceCard.tsx      # Node view: service card listing its tasks on that node
│   ├── RoleGroup.tsx            # Node view: draggable Managers / Workers group
│   ├── SwarmNodeBadges.tsx      # Role (leader), availability and state chips
│   ├── SwarmLinkEdge.tsx        # Node view: floating control-plane and overlay-network links
│   ├── GroupByToggle.tsx        # Network | Node graph grouping toggle
│   ├── StackSelector.tsx        # Header stack / compose project scope selector
│   ├── VolumeNode.tsx           # Volume node
│   ├── ElkEdge.tsx              # Custom edge with animated flow dots
│   ├── CanvasEdgeLayer.tsx      # Canvas-based edge renderer for large graphs
│   ├── ErrorBoundary.tsx        # React error boundary with fallback UI
│   ├── NodeHandles.tsx          # Shared source/target handles for nodes
│   ├── ViewTabs.tsx             # Graph / Table / Dashboard / Logs navigation tabs
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
│   │   ├── DetailPanelService.tsx   # Swarm service: mode, replicas, tasks, service logs
│   │   ├── DetailPanelSwarmNode.tsx # Swarm node: role, state, capacity, placed tasks
│   │   ├── DetailPanelVolume.tsx    # Volume detail (driver, usage, mounts)
│   │   ├── GhostVolumePanel.tsx     # Ghost (not-running) volume detail
│   │   ├── GhostNetworkPanel.tsx    # Ghost network detail
│   │   ├── GhostContainerPanel.tsx  # Ghost container detail
│   │   ├── GhostHeader.tsx         # Shared header for ghost resource panels
│   │   ├── ResourceHeader.tsx      # Shared header for live resource panels
│   │   ├── ContainerLink.tsx       # Clickable container name link
│   │   ├── ContainerList.tsx       # Container list within network/volume panels
│   │   ├── KeyValueList.tsx        # Generic key-value display component
│   │   ├── SecurityBadges.tsx      # Security capability badges
│   │   ├── StatusBadge.tsx         # Shared status pill (running, exited, not-running)
│   │   ├── Copyable.tsx            # Click-to-copy inline value wrapper
│   │   ├── panelStyles.ts          # Shared panel style helpers
│   │   └── shared.tsx              # Section, Row (click-to-copy), and style re-exports
│   ├── table/                   # Table view components
│   │   ├── TableView.tsx            # Resource tabs (Containers/Services/Networks/Volumes)
│   │   ├── GroupedTable.tsx         # Generic grouped table with toolbar and cards
│   │   ├── ContainerTable.tsx       # Container and swarm service table with stats sub-rows
│   │   ├── ContainerRow.tsx         # Container / service row with network colors and replica counts
│   │   ├── NetworkTable.tsx         # Network table with container counts
│   │   ├── NetworkRow.tsx           # Network row with subnet/gateway
│   │   ├── VolumeTable.tsx          # Volume table with mount paths
│   │   ├── VolumeRow.tsx            # Volume row with RTL path truncation
│   │   ├── SortableHeader.tsx       # Clickable sort column headers
│   │   ├── TableGroupHeader.tsx     # Collapsible group header with chevron
│   │   ├── TableToolbar.tsx         # Group-by dropdown toolbar
│   │   └── tableStyles.ts          # Shared table style constants
│   ├── dashboard/               # Dashboard view components
│       ├── Dashboard.tsx            # Dashboard layout with responsive grid
│       ├── DashboardCard.tsx        # Base card with loading/empty states
│       ├── StatusSummaryCard.tsx    # Container status breakdown
│       ├── ResourceChart.tsx        # uPlot time-series charts (CPU, memory, network, I/O)
│       ├── TopConsumersCard.tsx     # Top resource consumers table
│       ├── EventTimelineCard.tsx    # Color-coded Docker event timeline
│       ├── AlertsCard.tsx           # Container health alerts
│       ├── HostInfoCard.tsx         # Docker host system information
│       ├── DiskUsageCard.tsx        # Docker disk usage breakdown
│       ├── ImagesCard.tsx           # Docker image inventory
│       ├── ComposeProjectsCard.tsx  # Compose project / stack overview, click to scope
│       ├── SwarmNodesCard.tsx       # Swarm cluster overview with per-node CPU/memory bars
│       ├── TimeRangeSelector.tsx    # Time range selector (5m/1h/6h/24h)
│       ├── ProgressBar.tsx          # Reusable progress bar component
│       └── palette.ts              # Shared semantic and metric colors
│   ├── logs/                    # Global logs view components
│   │   ├── CommonLogs.tsx           # Unified all-container log view
│   │   ├── LogsToolbar.tsx          # Text/regex filter, chips, stream, pause, counts
│   │   └── LogRowMenu.tsx           # Per-row contextual filter actions
│   └── logwindow/               # Log rendering and floating window components
│       ├── LogStream.tsx            # Shared scroll/render/search log list (panel, windows, global view)
│       ├── LogWindow.tsx            # Movable, resizable, tabbed window chrome
│       ├── LogWindowLayer.tsx       # Overlay manager + drag-to-merge/detach hit-testing
│       └── LogDock.tsx              # Minimized-window dock strip
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
│   ├── useDetailPanel.ts           # Detail panel state management
│   ├── useServiceDetail.ts         # Swarm service inspect API integration
│   ├── useStackScope.ts            # Selected stack, synced to ?stack=
│   ├── useGroupBy.ts               # Graph grouping (network / node), synced to ?group=
│   ├── useTableSort.ts             # Generic column sort with toggle
│   ├── useTableGrouping.ts         # Group by compose/network/status/driver
│   ├── useRowHover.ts              # Shared row hover/selected state
│   ├── useLogs.ts                  # Log stream management (history + SSE, dedupe, buffer)
│   ├── useAggregateLogs.ts         # All-container merged log stream wrapper
│   ├── useLogFilters.ts            # Global-log filter chips + text/regex predicate
│   ├── useLogWindows.ts            # Floating log window state controller
│   ├── logWindowsState.ts          # Pure log-window state transitions
│   ├── useDragResize.ts            # Pointer-based window drag and resize
│   ├── usePollingFetch.ts          # Generic polling fetch with interval
│   ├── useStatsHistory.ts          # Stats time-series for dashboard charts (workload or node scope)
│   ├── useRecentEvents.ts          # Recent Docker events
│   ├── useSystemInfo.ts            # Docker host system information
│   ├── useDiskUsage.ts             # Docker disk usage
│   └── useImages.ts                # Docker image list
├── layout/                      # ELK layout pipeline
│   ├── elk.ts                      # Pipeline orchestrator
│   ├── nodeLayout.ts               # Fixed grid layout of the per-node view (no ELK)
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
│   ├── nodeTransform.ts            # Task placement and per-node view React Flow nodes
│   ├── swarmLinks.ts               # Control-plane and overlay-network links of the node view
│   ├── stack.ts                    # Stack membership, stack list and stack filtering helpers
│   ├── colors.ts                   # Deterministic network colors, status palette
│   ├── constants.ts                # Shared layout and visual constants
│   ├── deltaUtils.ts               # WebSocket delta update merging
│   ├── formatBytes.ts              # Byte formatting utility
│   ├── format.ts                   # General formatting helpers
│   ├── alerts.ts                   # Dashboard alert generation logic
│   ├── logParser.ts                # Docker log stream parser
│   ├── nodeStyles.ts               # Ghost border and left-rail accent helpers for nodes
│   ├── pathUtils.ts                # SVG path utilities
│   └── selectionGraph.ts           # Click-to-highlight graph traversal
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
Backend (WebSocket) → useDockGraph → App (stack scope) → FlowCanvas
                                              ↓
                    ┌─────────────────────────┼─────────────────────────┐
                    │                         │                         │
               Graph View                Table View              Dashboard View
                    │                         │                         │
         network: graphTransform       useTableSort +           useStatsHistory +
                  → ELK               useTableGrouping        useRecentEvents + ...
           node: nodeTransform                │                         │
            → nodeLayout grid       GroupedTable renders         dashboard cards
                    │                         │                         │
           React Flow renders                 │                         │
                    └────────────┬────────────┘                         │
                                 │                                      │
                      Detail Panels (shared)                    REST API polling
                       useContainerDetail /                 /api/stats/history + ...
                      useServiceDetail / ...
```

1. **`useDockGraph`** connects to `/ws`, receives an initial snapshot, then incremental delta updates as containers start/stop. Auto-reconnects with exponential backoff.

2. **Stack scope**: `useStackScope` holds the selected stack (compose project or swarm stack, or standalone containers) in `?stack=`. `App` filters the snapshot and the live stats before they reach any view, so graph, table, dashboard and logs all show the same scope; the logs view and stats history also pass the stack to the backend.

3. **Graph view**: `graphTransform` converts the backend's flat node/edge model into React Flow's hierarchical structure — containers are grouped inside network groups, volumes are placed in the group of their first consumer. Swarm services render as `ServiceNode`s inside their overlay networks. The ELK layout pipeline then computes positions.

4. **Node view** (swarm): with `?group=node` (`useGroupBy`, `GroupByToggle`), `nodeTransform` places each running task on its node and builds one `SwarmNodeGroup` per node, with a `NodeServiceCard` per service, inside `RoleGroup`s for managers and workers. `nodeLayout` positions them on a fixed grid (managers first, workers wrapped in rows), and `swarmLinks` adds the Managers → Workers control-plane link and, for a selected service, links to services sharing an overlay network on other nodes.

5. **Table view**: `TableView` splits nodes by type into resource tabs; the Services tab appears when the graph has swarm services. Each table uses `useTableGrouping` (compose project / stack, network, status, or driver) and `useTableSort` for interactive column sorting. `GroupedTable` provides the shared rendering pattern.

6. **Dashboard view**: A 13-card monitoring dashboard that polls REST endpoints for stats history, recent events, system info, disk usage, and images. Cards include CPU/memory/network/I/O time-series charts, top resource consumers, event timeline, alerts, and infrastructure overview. In swarm mode a `SwarmNodesCard` shows every node with its tasks and resource use, and the charts can switch to per-node series (`?scope=nodes` on the stats history API).

7. **Global logs view**: `CommonLogs` merges every container's logs into one time-ordered stream via `useAggregateLogs` (backed by `/api/logs` SSE + `/api/logs/history`). `useLogFilters` drives client-side filtering — text (literal or regex), per-container include/exclude, and stream — surfaced as removable chips and per-row actions, with deep back-scroll, a pause toggle, and a Ctrl+F find. Lines render through the shared `LogStream` with color-coded container badges.

8. **Pop-out log windows**: Any container's logs can open in a floating window managed by `useLogWindows`. Windows drag and resize (`useDragResize`), merge into tabs when dropped together, detach back out, and minimize into a dock — all reusing `LogStream` for rendering.

9. **Detail panels**: Clicking any resource in graph or table view opens a detail panel. Container panels fetch live stats, logs, and inspect data. Network and volume panels show IPAM configuration and mount relationships. Service panels list tasks with their nodes and stream service logs; swarm node panels show role, state, capacity and the tasks placed on the node.

### Key Design Decisions

- **ELK over dagre/d3-force**: ELK handles hierarchical grouping (containers inside network groups) natively and produces clean orthogonal edge routing. Force-directed layouts don't respect parent-child containment.

- **Fixed grid for the node view**: the per-node view has no dependency edges to route, so `nodeLayout` places boxes on a deterministic grid instead of running ELK; positions stay stable as tasks move.

- **URL-synced view state**: the stack scope (`?stack=`) and graph grouping (`?group=`) live in the URL, so a scoped view survives reloads and can be shared.

- **Canvas-based edge rendering**: For large graphs, edges are rendered on a `<canvas>` element with level-of-detail optimization instead of individual SVG elements.

- **Shared GroupedTable**: All resource tables compose a single `GroupedTable` component, avoiding duplication of toolbar, group cards, sort headers, and empty states.

- **Delta updates**: After the initial snapshot, the backend sends only changed nodes/edges. The `useDockGraph` hook merges deltas into the current state to minimize re-renders.

- **Dashboard polling with caching**: Dashboard cards use `usePollingFetch` for periodic REST calls. The backend caches system info and disk usage responses to avoid hammering the Docker API on every client poll.

- **Theme tokens**: All colors reference theme context values — no hardcoded colors. Table-specific tokens (`rowHover`, `cardBg`, `statsRowBg`) were added to avoid magic values.
