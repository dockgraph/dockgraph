---
applyTo: "frontend/src/**/*.ts,frontend/src/**/*.tsx"
---

# Frontend Review Rules (React + TypeScript)

## TypeScript

- Strict mode is enabled (`"strict": true` in tsconfig). Do not weaken type
  checking with `any` unless absolutely necessary — prefer `unknown` with
  type narrowing.
- Use `interface` for object shapes and `type` for unions/intersections.
- Domain types are centralized in `src/types.ts`. Do not define inline types
  that duplicate existing definitions.
- Use `import type` for type-only imports (enforced by `verbatimModuleSyntax`).

## React Patterns

- Functional components only. No class components.
- Custom hooks go in `src/hooks/`. Each hook should have a single, clear
  responsibility.
- Avoid `useEffect` for derived state — compute during render instead.
- Memoize with `useMemo`/`useCallback` only when there's a measurable
  performance benefit, not by default.

## Performance

- The graph can contain dozens of nodes updating in real time. Watch for:
  - Unnecessary re-renders caused by new object/array references on every render.
  - Missing or incorrect dependency arrays in hooks.
  - Heavy computations that should be memoized (ELK layout calculations).
- The `snapshotFingerprint` pattern in `useDockGraph` prevents redundant
  re-renders — similar guards should be used for expensive state updates.
- Table views with many rows: `useTableSort` and `useTableGrouping` recompute
  on every state change — verify sort/group functions are memoized correctly.
- Detail panel hooks (`useContainerDetail`, `useContainerLogs`) open HTTP/SSE
  connections — verify cleanup on unmount.

## Graph Rendering Pipeline

The layout pipeline flows through several modules:

1. `hooks/useDockGraph.ts` — WebSocket state management
2. `layout/elkGraph.ts` — converts domain types to ELK input
3. `layout/elk.ts` — runs the ELK layout algorithm
4. `layout/elkPositions.ts` — maps ELK output to React Flow positions
5. `layout/edgePaths.ts` — computes SVG edge paths
6. `components/` — renders nodes and edges

Changes to any step can affect downstream rendering. Review layout changes
with particular care for edge cases (empty graphs, single nodes, disconnected
components).

## Table View Pipeline

Table components compose a shared `GroupedTable`:

1. `hooks/useTableGrouping.ts` — groups nodes by compose project, network, status, or driver
2. `hooks/useTableSort.ts` — generic column sort with direction toggle
3. `components/table/GroupedTable.tsx` — renders toolbar, group cards, sort headers, rows
4. Each resource table (`ContainerTable`, `NetworkTable`, `VolumeTable`) provides
   columns, sort key functions, group options, and a row renderer.

## Detail Panels

The `panels/` directory contains resource-specific detail components:

- `DetailPanel.tsx` — slide-in container with close button
- `DetailPanelStats.tsx` — live CPU/memory/network stats (via `useContainerStats`)
- `DetailPanelLogs.tsx` — streaming logs (via `useContainerLogs`)
- `GhostVolumePanel.tsx`, `GhostNetworkPanel.tsx`, `GhostContainerPanel.tsx` — panels for compose-defined but not-running resources

Each panel fetches its own data via dedicated hooks. Ghost panels render
compose-parsed data only (no runtime APIs).

## Styling

- Theme support (dark/light) is implemented via React context (`src/theme.tsx`).
- Colors for network groups are generated deterministically from network names
  (`src/utils/colors.ts`).
- Avoid hardcoded color values — use theme-aware values.
- Table styles are centralized in `components/table/tableStyles.ts`. Theme tokens
  include `rowHover`, `cardBg`, and `statsRowBg` for table-specific styling.
- Shared panel styles (`monoStyle`, `navLinkStyle`) live in `panels/panelStyles.ts`.

## WebSocket Integration

- The frontend connects to the backend at `/ws` using native WebSocket.
- Reconnection uses exponential backoff capped at 30 seconds.
- Both `snapshot` (full state) and `delta` (incremental) message types must
  be handled correctly. Missing delta handling causes stale UI.

## Dependencies

- Core: React 19, React Flow (`@xyflow/react`), ELK.js.
- Keep the dependency footprint small. Avoid adding utility libraries for
  things that can be done with a few lines of plain TypeScript.
