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

## Styling

- Theme support (dark/light) is implemented via React context (`src/theme.tsx`).
- Colors for network groups are generated deterministically from network names
  (`src/utils/colors.ts`).
- Avoid hardcoded color values — use theme-aware values.

## WebSocket Integration

- The frontend connects to the backend at `/ws` using native WebSocket.
- Reconnection uses exponential backoff capped at 30 seconds.
- Both `snapshot` (full state) and `delta` (incremental) message types must
  be handled correctly. Missing delta handling causes stale UI.

## Dependencies

- Core: React 19, React Flow (`@xyflow/react`), ELK.js.
- Keep the dependency footprint small. Avoid adding utility libraries for
  things that can be done with a few lines of plain TypeScript.
