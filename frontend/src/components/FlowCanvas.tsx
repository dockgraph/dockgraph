import { useCallback, useEffect, useMemo, useRef, useState, lazy, Suspense } from "react";
import {
  ReactFlow,
  MiniMap,
  Panel,
  Controls,
  Background,
  BackgroundVariant,
  useReactFlow,
  type Node as RFNode,
  type Edge as RFEdge,
} from "@xyflow/react";
import { ANIMATION_NODE_LIMIT, DETAIL_PANEL_WIDTH, Z } from "../utils/constants";
import { SWARM_LINK_EDGE_TYPE, controlLinkEdges, overlayLinkEdges } from "../utils/swarmLinks";
import { serviceIdOf, type SelectionState } from "../utils/selectionGraph";

import { ContainerNode } from "./ContainerNode";
import { ServiceNode } from "./ServiceNode";
import { StackSelector } from "./StackSelector";
import { GroupByToggle } from "./GroupByToggle";
import { NetworkGroup } from "./NetworkGroup";
import { SwarmNodeGroup } from "./SwarmNodeGroup";
import { NodeServiceCard } from "./NodeServiceCard";
import { RoleGroup } from "./RoleGroup";
import { ControlSummary } from "./ControlSummary";
import { VolumeNode } from "./VolumeNode";
import { ElkEdge } from "./ElkEdge";
import { SwarmLinkEdge } from "./SwarmLinkEdge";
import { CanvasEdgeLayer, type CanvasEdgeLayerHandle } from "./CanvasEdgeLayer";
import { ThemeToggle } from "./ThemeToggle";
import { Brand } from "./Brand";
import { LogoutButton } from "./LogoutButton";
import { StatusIndicator } from "./StatusIndicator";
import { SearchFilter } from "./SearchFilter";
import { ViewTabs } from "./ViewTabs";
import type { ViewKey } from "./ViewTabs";
import type { ResourceTab } from "./table/TableView";
import { Spinner } from "./StateDisplay";
import { DetailPanel } from "./panels/DetailPanel";
import { DetailPanelHeader } from "./panels/DetailPanelHeader";
import { DetailPanelStats } from "./panels/DetailPanelStats";
import { DetailPanelProcess } from "./panels/DetailPanelProcess";
import { DetailPanelPorts } from "./panels/DetailPanelPorts";
import { DetailPanelMounts } from "./panels/DetailPanelMounts";
import { DetailPanelEnv } from "./panels/DetailPanelEnv";
import { DetailPanelLabels } from "./panels/DetailPanelLabels";
import { DetailPanelNetwork } from "./panels/DetailPanelNetwork";
import { DetailPanelSecurity } from "./panels/DetailPanelSecurity";
import { DetailPanelHealth } from "./panels/DetailPanelHealth";
import { DetailPanelLogs } from "./panels/DetailPanelLogs";
import { DetailPanelVolume } from "./panels/DetailPanelVolume";
import { DetailPanelNetworkInfo } from "./panels/DetailPanelNetworkInfo";
import { GhostVolumePanel } from "./panels/GhostVolumePanel";
import { GhostNetworkPanel } from "./panels/GhostNetworkPanel";
import { GhostContainerPanel } from "./panels/GhostContainerPanel";
import { ResourceHeader } from "./panels/ResourceHeader";
import { GhostHeader } from "./panels/GhostHeader";
import { ContainerList } from "./panels/ContainerList";
import { DetailPanelService, DetailPanelServiceHeader } from "./panels/DetailPanelService";
import { DetailPanelSwarmNode, DetailPanelSwarmNodeHeader } from "./panels/DetailPanelSwarmNode";
import { useGraphLayout } from "../hooks/useGraphLayout";
import { useSelectionHighlight } from "../hooks/useSelectionHighlight";
import { useDetailPanel } from "../hooks/useDetailPanel";
import { useLogWindows } from "../hooks/useLogWindows";
import { useSearchFilter } from "../hooks/useSearchFilter";
import { useGroupBy } from "../hooks/useGroupBy";
import { useSystemInfo } from "../hooks/useSystemInfo";
import { networkColor, stackColor, swarmRoleColor } from "../utils/colors";
import { nodeStatsKey, projectOf, resolveNodeRef, taskContainerName, type StackSummary } from "../utils/stack";
import { useTheme } from "../theme";
import type {
  DGNode, DGEdge, ContainerStatsData, SwarmNodeGroupData, NodeServiceCardData, RoleGroupData,
} from "../types";
import type { ReactNode } from "react";

// Code-split the Table and Dashboard views (the latter pulls in uPlot) so the
// default Graph view doesn't pay for them on first load.
const TableView = lazy(() => import("./table/TableView").then((m) => ({ default: m.TableView })));
const Dashboard = lazy(() => import("./dashboard/Dashboard").then((m) => ({ default: m.Dashboard })));
const CommonLogs = lazy(() => import("./logs/CommonLogs").then((m) => ({ default: m.CommonLogs })));
// Code-split the floating log windows so they cost nothing until first opened.
const LogWindowLayer = lazy(() => import("./logwindow/LogWindowLayer"));

function ViewFallback() {
  return (
    <div style={{ position: "absolute", inset: 0, top: 50, display: "grid", placeItems: "center" }}>
      <Spinner />
    </div>
  );
}

function Overlay({ children }: { children: ReactNode }) {
  return (
    <div
      role="status"
      aria-live="polite"
      style={{
        position: "absolute",
        inset: 0,
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        zIndex: 5,
        pointerEvents: "none",
      }}
    >
      {children}
    </div>
  );
}

const nodeTypes = {
  containerNode: ContainerNode,
  serviceNode: ServiceNode,
  networkGroup: NetworkGroup,
  volumeNode: VolumeNode,
  nodeGroup: SwarmNodeGroup,
  roleGroup: RoleGroup,
  nodeServiceCard: NodeServiceCard,
  controlSummary: ControlSummary,
};

const edgeTypes = {
  elk: ElkEdge,
  [SWARM_LINK_EDGE_TYPE]: SwarmLinkEdge,
};

interface FlowCanvasProps {
  dgNodes: DGNode[];
  dgEdges: DGEdge[];
  connected: boolean;
  ready: boolean;
  statsMap: Map<string, ContainerStatsData>;
  /** Every stack in the unscoped graph, for the header stack selector. */
  stacks?: StackSummary[];
  /** Stack the graph is scoped to (null = all). dgNodes/dgEdges are already filtered. */
  selectedStack?: string | null;
  onSelectStack?: (stack: string | null) => void;
}

export function FlowCanvas({
  dgNodes,
  dgEdges,
  connected,
  ready,
  statsMap,
  stacks = [],
  selectedStack = null,
  onSelectStack,
}: FlowCanvasProps) {
  const { theme } = useTheme();
  const canvasEdgeRef = useRef<CanvasEdgeLayerHandle>(null);
  const selectNodeRef = useRef<((id: string) => void) | undefined>(undefined);

  // Graph grouping: by network (default) or by swarm node. The per-node view
  // only exists when the graph has swarm nodes; otherwise a stale ?group=node
  // falls back to the network view.
  const { groupBy, setGroupBy } = useGroupBy();
  const hasSwarmNodes = dgNodes.some((n) => n.type === "swarmnode");
  const effectiveGroupBy = hasSwarmNodes ? groupBy : "network";
  const { data: systemInfo } = useSystemInfo();
  const localNodeId = systemInfo?.swarm?.nodeId ?? null;

  const {
    nodes,
    edges,
    onNodesChange,
    onEdgesChange,
    layoutBusy,
    layoutError,
  } = useGraphLayout(dgNodes, dgEdges, theme.edgeStroke, theme.edgeSignal, effectiveGroupBy, localNodeId);

  // Fit the viewport once the layout of a newly chosen grouping has settled
  // (the ReactFlow `fitView` prop only applies to the first render). fitView
  // is queued by React Flow until the new nodes are measured.
  const { fitView } = useReactFlow();
  const fittedGroupByRef = useRef(effectiveGroupBy);
  useEffect(() => {
    if (layoutBusy || fittedGroupByRef.current === effectiveGroupBy) return;
    fittedGroupByRef.current = effectiveGroupBy;
    void fitView({ duration: 300 });
  }, [effectiveGroupBy, layoutBusy, fitView]);

  // Service hovered in the per-node view: its cards on every node get outlined.
  const [hoveredServiceId, setHoveredServiceId] = useState<string | null>(null);
  const handleNodeMouseEnter = useCallback((_: React.MouseEvent, node: RFNode) => {
    if (node.type === "nodeServiceCard") {
      setHoveredServiceId((node.data as unknown as NodeServiceCardData).serviceId);
    }
  }, []);
  const handleNodeMouseLeave = useCallback((_: React.MouseEvent, node: RFNode) => {
    if (node.type === "nodeServiceCard") setHoveredServiceId(null);
  }, []);

  // Ids on the canvas, so a detail click only selects what is actually drawn
  // (a swarm node opened from the dashboard has no box in the network view).
  const canvasIdsRef = useRef<Set<string>>(new Set());
  useEffect(() => { canvasIdsRef.current = new Set(nodes.map((n) => n.id)); }, [nodes]);

  const hasVisibleNodes = effectiveGroupBy === "node" || dgNodes.some(
    (n) => n.type === "container" || n.type === "service" || n.type === "volume",
  );
  const showEmptyState = !ready || !hasVisibleNodes;
  const largeGraph = dgNodes.length > ANIMATION_NODE_LIMIT;

  // View navigation state.
  const [activeView, setActiveView] = useState<ViewKey>("graph");
  const [tableTab, setTableTab] = useState<ResourceTab>("containers");

  // Detail panel state.
  const {
    detailNodeId,
    detailOpen,
    variant,
    detailDgNode,
    groupContainers,
    volumeMounts,
    containerData,
    volumeData,
    networkData,
    serviceData,
    loading: detailLoading,
    error: detailError,
    handleInfoClick,
    handleNavigate,
    closeDetail,
  } = useDetailPanel(dgNodes, dgEdges);

  // Wire info click to also select the node in the graph.
  const handleInfoClickWithSelect = useCallback(
    (nodeId: string) => {
      handleInfoClick(nodeId);
      if (canvasIdsRef.current.has(nodeId)) selectNodeRef.current?.(nodeId);
    },
    [handleInfoClick],
  );

  // Per-node service cards (and their task rows) open the service's panel.
  // The service has no node of its own in that view, so the clicked card
  // stays the selection.
  const handleTaskInfoClick = useCallback(
    (serviceId: string) => handleInfoClick(resolveNodeRef(dgNodes, serviceId)),
    [dgNodes, handleInfoClick],
  );

  // Floating log windows. `openContainerInfo` re-opens the side panel for a
  // container (windows store the bare name; the panel expects a node id).
  const logWindows = useLogWindows();
  const openContainerInfo = useCallback(
    (containerId: string) => handleInfoClickWithSelect(resolveNodeRef(dgNodes, `container:${containerId}`)),
    [dgNodes, handleInfoClickWithSelect],
  );

  // Names of the workloads in scope, used by the logs view to narrow the
  // standalone scope client-side (the backend only filters named stacks).
  const scopeNames = useMemo(
    () => new Set(dgNodes.filter((n) => n.type === "container" || n.type === "service").map((n) => n.name)),
    [dgNodes],
  );

  // Inject live stats and info callback into container and volume node data (doesn't affect layout).
  const enrichedNodes = useMemo(() => {
    return nodes.map((n) => {
      // Services are keyed by service name in the stats stream (per-service
      // aggregates), which is also their node name.
      if (n.type === "containerNode" || n.type === "serviceNode") {
        const dgNode = (n.data as { dgNode: DGNode }).dgNode;
        const s = statsMap.get(dgNode.name);
        return {
          ...n,
          data: { ...n.data, stats: s, onInfoClick: handleInfoClickWithSelect },
        };
      }
      if (n.type === "volumeNode") {
        return { ...n, data: { ...n.data, onInfoClick: handleInfoClickWithSelect } };
      }
      if (n.type === "networkGroup") {
        return { ...n, data: { ...n.data, onInfoClick: handleInfoClickWithSelect } };
      }
      // Swarm node boxes carry the node aggregate (`node:{hostname}`); task
      // cards the per-task entry reported by the node agent.
      if (n.type === "nodeGroup") {
        const d = n.data as unknown as SwarmNodeGroupData;
        const stats = d.unassigned ? undefined : statsMap.get(nodeStatsKey(d.dgNode.name));
        return { ...n, data: { ...n.data, stats, onInfoClick: handleInfoClickWithSelect } };
      }
      if (n.type === "nodeServiceCard") {
        const d = n.data as unknown as NodeServiceCardData;
        const taskStats: Record<string, ContainerStatsData> = {};
        for (const t of d.tasks) {
          const s = statsMap.get(taskContainerName(d.serviceName, t));
          if (s) taskStats[t.id] = s;
        }
        return {
          ...n,
          data: {
            ...n.data,
            taskStats,
            peerHover: hoveredServiceId !== null && d.serviceId === hoveredServiceId,
            onInfoClick: handleTaskInfoClick,
          },
        };
      }
      return n;
    });
  }, [nodes, statsMap, handleInfoClickWithSelect, handleTaskInfoClick, hoveredServiceId]);

  // Dashboard cards reference workloads as `container:{name}`; resolve those
  // to the real node (a swarm service, or a prefixed compose name).
  const handleInspectRef = useCallback(
    (ref: string) => handleInfoClickWithSelect(resolveNodeRef(dgNodes, ref)),
    [dgNodes, handleInfoClickWithSelect],
  );
  // Every stack change (header selector or dashboard) closes the detail
  // panel: the resource it shows may be out of the new scope, and it would
  // otherwise keep fetching that resource's details and logs.
  const changeStack = useCallback(
    (stack: string | null) => {
      if (stack === selectedStack) return;
      closeDetail();
      onSelectStack?.(stack);
    },
    [closeDetail, onSelectStack, selectedStack],
  );
  // Dashboard drill-down: scope to the stack and show it in the graph.
  const handleSelectStack = useCallback(
    (stack: string) => {
      changeStack(stack);
      setActiveView("graph");
    },
    [changeStack],
  );

  // Search & filter.
  const search = useSearchFilter(dgNodes);

  // Dashboard drill-downs into the table view. Each clears any stale selection
  // and filter first so the destination is predictable. The table and the
  // search bar share this `search` state, so an applied status filter (and its
  // match count) surface in the search bar automatically.
  const { clearAll, toggleStatus } = search;
  const handleStatusFilter = useCallback(
    (status: string, tab: ResourceTab) => {
      closeDetail();
      clearAll();
      toggleStatus(status);
      setTableTab(tab);
      setActiveView("table");
    },
    [closeDetail, clearAll, toggleStatus],
  );
  const handleResourceTab = useCallback(
    (tab: ResourceTab) => {
      closeDetail();
      clearAll();
      setTableTab(tab);
      setActiveView("table");
    },
    [closeDetail, clearAll],
  );

  // Per-node view links: the Managers → Workers control spokes always, plus
  // overlay links from a selected service card to its network peers.
  const controlLinks = useMemo(
    () => (effectiveGroupBy === "node" ? controlLinkEdges(dgNodes) : []),
    [effectiveGroupBy, dgNodes],
  );
  const linkEdgesFor = useCallback(
    (selection: SelectionState | null): RFEdge[] => {
      if (effectiveGroupBy !== "node") return [];
      const links = [...controlLinks];
      const selected = selection?.type === "node" ? nodes.find((n) => n.id === selection.id) : undefined;
      const serviceId = serviceIdOf(selected);
      if (serviceId) links.push(...overlayLinkEdges(dgNodes, dgEdges, nodes, serviceId));
      return links;
    },
    [effectiveGroupBy, controlLinks, nodes, dgNodes, dgEdges],
  );

  const {
    styledNodes,
    styledEdges,
    canvasEdges,
    svgEdges,
    linkEdges,
    onNodeClick,
    onEdgeClick,
    onPaneClick,
    selectNode,
    selectEdge,
  } = useSelectionHighlight(
    enrichedNodes,
    edges,
    largeGraph,
    search.matchingNodeIds,
    linkEdgesFor,
  );
  useEffect(() => { selectNodeRef.current = selectNode; }, [selectNode]);
  // Swarm links need live node positions, so they are always SVG edges.
  const displayedEdges = useMemo(
    () => {
      const base = largeGraph ? svgEdges : styledEdges;
      return linkEdges.length > 0 ? [...base, ...linkEdges] : base;
    },
    [largeGraph, svgEdges, styledEdges, linkEdges],
  );

  // Canvas edge hit-test helper — returns the edge if hit, null otherwise.
  const canvasEdgeHit = useCallback(
    (event: React.MouseEvent): RFEdge | null => {
      if (!largeGraph || !canvasEdgeRef.current) return null;
      const hitId = canvasEdgeRef.current.hitTest(event.clientX, event.clientY);
      if (!hitId) return null;
      return edges.find((e) => e.id === hitId) ?? null;
    },
    [largeGraph, edges],
  );

  // For large graphs, intercept node clicks to check if the click actually
  // hit a canvas edge passing through the node area (common with group nodes).
  const handleNodeClick = useCallback(
    (event: React.MouseEvent, node: RFNode) => {
      const edgeHit = canvasEdgeHit(event);
      if (edgeHit) {
        onEdgeClick(event, edgeHit);
      } else {
        onNodeClick(event, node);
      }
    },
    [canvasEdgeHit, onEdgeClick, onNodeClick],
  );

  // Listen for edge click events dispatched from ElkEdge components.
  useEffect(() => {
    const handler = (e: Event) => {
      selectEdge((e as CustomEvent).detail);
    };
    document.addEventListener('dg:edge-click', handler);
    return () => document.removeEventListener('dg:edge-click', handler);
  }, [selectEdge]);

  // Remove React Flow's "nopan" class from edge wrappers so d3-zoom allows
  // panning when dragging over edges. Only needed for SVG edges (small graphs).
  useEffect(() => {
    if (largeGraph) return;
    document.querySelectorAll('.react-flow__edge.nopan').forEach((el) => {
      el.classList.remove('nopan');
    });
  }, [edges, largeGraph]);

  // Pane click handler: test canvas edges first, then clear selection.
  const handlePaneClick = useCallback(
    (event: React.MouseEvent) => {
      const edgeHit = canvasEdgeHit(event);
      if (edgeHit) {
        onEdgeClick(event, edgeHit);
      } else {
        onPaneClick();
      }
    },
    [canvasEdgeHit, onEdgeClick, onPaneClick],
  );

  return (
    <div
      style={{
        width: "100%",
        height: "100%",
        position: "relative",
        overflow: "hidden",
        background: theme.canvasBg,
      }}
    >
      <style>{'.react-flow__edge { cursor: default; }'}</style>

      <DetailPanel
        open={detailOpen}
        onClose={closeDetail}
        loading={detailLoading}
        error={detailError}
        header={
          variant.kind === 'group' ? (
            <ResourceHeader name="Unmanaged" subtitle="Default bridge network" theme={theme} />
          ) : variant.kind === 'ghost-container' || variant.kind === 'ghost-volume' || variant.kind === 'ghost-network' || variant.kind === 'ghost-service' ? (
            <GhostHeader node={variant.node} theme={theme} />
          ) : (variant.kind === 'network' || variant.kind === 'volume') && detailDgNode ? (
            <ResourceHeader name={detailDgNode.name} subtitle={detailDgNode.driver} theme={theme} />
          ) : variant.kind === 'swarmnode' ? (
            <DetailPanelSwarmNodeHeader node={variant.node} />
          ) : variant.kind === 'service' && serviceData ? (
            <DetailPanelServiceHeader detail={serviceData} />
          ) : containerData ? (
            <DetailPanelHeader detail={containerData} />
          ) : null
        }
      >
        {variant.kind === 'group' ? (
          <>
            <div style={{ fontSize: 11, color: theme.nodeSubtext, marginBottom: 10 }}>
              Containers not assigned to a named network.
            </div>
            <ContainerList containers={groupContainers} onNavigate={handleNavigate} theme={theme} />
          </>
        ) : variant.kind === 'ghost-volume' ? (
          <GhostVolumePanel node={variant.node} mounts={volumeMounts} onNavigate={handleNavigate} />
        ) : variant.kind === 'ghost-network' ? (
          <GhostNetworkPanel node={variant.node} containers={groupContainers} onNavigate={handleNavigate} />
        ) : variant.kind === 'ghost-container' || variant.kind === 'ghost-service' ? (
          <GhostContainerPanel node={variant.node} onNavigate={handleNavigate} />
        ) : variant.kind === 'swarmnode' ? (
          <DetailPanelSwarmNode node={variant.node} dgNodes={dgNodes} statsMap={statsMap} onNavigate={handleNavigate} />
        ) : variant.kind === 'service' ? (
          serviceData ? (
            <DetailPanelService detail={serviceData} statsMap={statsMap} active={detailOpen} onNavigate={handleNavigate} />
          ) : null
        ) : variant.kind === 'network' && networkData ? (
          <DetailPanelNetworkInfo network={networkData} onNavigate={handleNavigate} />
        ) : variant.kind === 'volume' && volumeData ? (
          <DetailPanelVolume volume={volumeData} mounts={volumeMounts} onNavigate={handleNavigate} />
        ) : containerData ? (
          <>
            <DetailPanelStats stats={variant.kind === 'container' ? statsMap.get(containerData.name) : undefined} />
            <DetailPanelProcess detail={containerData} />
            <DetailPanelPorts ports={containerData.ports} />
            <DetailPanelMounts mounts={containerData.mounts} onNavigate={handleNavigate} />
            <DetailPanelNetwork networkMode={containerData.networkMode} networks={containerData.networks} onNavigate={handleNavigate} />
            <DetailPanelSecurity security={containerData.security} />
            <DetailPanelEnv env={containerData.env} />
            <DetailPanelLabels labels={containerData.labels} />
            <DetailPanelHealth health={containerData.health} />
            <DetailPanelLogs
              containerId={variant.kind === 'container' ? variant.containerName : null}
              active={detailOpen}
              onPopOut={
                variant.kind === 'container'
                  ? () => logWindows.openLogs(variant.containerName, containerData?.name ?? variant.containerName)
                  : undefined
              }
            />
          </>
        ) : null}
      </DetailPanel>

      {/* Canvas area — shrinks when the detail panel is open so that
          React Flow's viewport calculations (fitView, center, etc.)
          use the actual visible area rather than the full window. */}
      <div
        style={{
          position: "absolute",
          top: 0,
          left: 0,
          right: detailOpen ? DETAIL_PANEL_WIDTH : 0,
          bottom: 0,
        }}
      >
      <div
        style={{
          position: "absolute",
          top: 0,
          left: 0,
          right: 0,
          height: 50,
          zIndex: Z.header,
          display: "flex",
          alignItems: "center",
          gap: 12,
          padding: "0 12px",
          background: theme.panelBg,
          borderBottom: `1px solid ${theme.panelBorder}`,
        }}
      >
        <div style={{ flex: "1 1 0", display: "flex", alignItems: "center", gap: 12 }}>
          <Brand />
          <span
            aria-hidden="true"
            style={{ width: 1, height: 20, background: theme.panelBorder, flex: "0 0 auto" }}
          />
          <ViewTabs activeView={activeView} onViewChange={setActiveView} />
          {onSelectStack && (
            <StackSelector stacks={stacks} selected={selectedStack} onSelect={changeStack} />
          )}
          {activeView === "graph" && hasSwarmNodes && (
            <GroupByToggle value={groupBy} onChange={setGroupBy} />
          )}
        </div>
        <SearchFilter search={search} />
        <div style={{ flex: "1 1 0", display: "flex", alignItems: "center", justifyContent: "flex-end", gap: 8 }}>
          <LogoutButton />
          <StatusIndicator connected={connected} />
        </div>
      </div>

      {activeView === "table" ? (
        <div style={{ position: "absolute", inset: 0, top: 50, display: "flex", flexDirection: "column" }}>
          <Suspense fallback={<ViewFallback />}>
            <TableView
              nodes={dgNodes}
              edges={dgEdges}
              statsMap={statsMap}
              matchingNodeIds={search.matchingNodeIds}
              selectedNodeId={detailNodeId}
              onRowClick={handleInfoClickWithSelect}
              activeTab={tableTab}
              onTabChange={setTableTab}
            />
          </Suspense>
        </div>
      ) : activeView === "dashboard" ? (
        <Suspense fallback={<ViewFallback />}>
          <Dashboard
            nodes={dgNodes}
            statsMap={statsMap}
            onStatusFilter={handleStatusFilter}
            onResourceTab={handleResourceTab}
            onInspect={handleInspectRef}
            stack={selectedStack}
            onSelectStack={onSelectStack ? handleSelectStack : undefined}
          />
        </Suspense>
      ) : activeView === "logs" ? (
        <Suspense fallback={<ViewFallback />}>
          <CommonLogs
            key={selectedStack ?? ''}
            active={activeView === "logs"}
            onOpenContainer={openContainerInfo}
            stack={selectedStack}
            scopeNames={scopeNames}
          />
        </Suspense>
      ) : (
      <div style={{ position: "absolute", inset: 0, top: 50 }}>
      {showEmptyState && (
        <Overlay>
          <p style={{ color: theme.nodeSubtext, fontSize: 14 }}>
            {!ready
              ? "Connecting to backend..."
              : selectedStack
                ? "Nothing to show in this stack. Pick another stack or \"All stacks\" above."
                : "No containers detected. Start a container to visualize the graph."}
          </p>
        </Overlay>
      )}

      {layoutBusy && !showEmptyState && (
        <Overlay>
          <p style={{ color: theme.nodeSubtext, fontSize: 14 }}>
            Computing layout for {dgNodes.length} nodes...
          </p>
        </Overlay>
      )}

      {layoutError && !showEmptyState && (
        <Overlay>
          <p style={{ color: theme.nodeSubtext, fontSize: 14 }}>
            Layout computation failed. Try reloading the page.
          </p>
        </Overlay>
      )}

      {largeGraph && (
        <CanvasEdgeLayer ref={canvasEdgeRef} edges={canvasEdges} />
      )}

      <ReactFlow
        nodes={styledNodes}
        edges={displayedEdges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        onNodeClick={largeGraph ? handleNodeClick : onNodeClick}
        onPaneClick={handlePaneClick}
        onNodeMouseEnter={handleNodeMouseEnter}
        onNodeMouseLeave={handleNodeMouseLeave}
        nodeTypes={nodeTypes}
        edgeTypes={edgeTypes}
        // Only nodes that opt in via `draggable: true` move: the per-node
        // view's role groups. Everything else (incl. the network view) stays
        // fixed, and dragging on a fixed node or the pane pans the canvas.
        nodesDraggable={false}
        nodesConnectable={false}
        elevateNodesOnSelect={false}
        onlyRenderVisibleElements={largeGraph}
        fitView
        minZoom={0.05}
        maxZoom={2}
        style={{ background: theme.canvasBg }}
      >
        {!largeGraph && (
          <>
            <Background
              id="dots-minor"
              variant={BackgroundVariant.Dots}
              color={theme.dotColor}
              gap={20}
              size={1}
            />
            <Background
              id="dots-major"
              variant={BackgroundVariant.Dots}
              color={theme.dotColorMajor}
              gap={100}
              size={2}
            />
          </>
        )}
        <Panel
          position="bottom-left"
          style={{
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
            gap: 10,
            margin: 15,
          }}
        >
          <Controls
            showInteractive={false}
            position="bottom-left"
            style={{ position: "relative" }}
          />
          <ThemeToggle />
        </Panel>
        {!largeGraph && (
          <MiniMap
            style={{
              background: theme.minimapBg,
              border: `1px solid ${theme.panelBorder}`,
            }}
            maskColor={theme.minimapMask}
            nodeColor={(node) => {
              if (node.type === "networkGroup") {
                return (
                  networkColor(
                    (node.data as { dgNode: DGNode }).dgNode.name,
                    projectOf((node.data as { dgNode: DGNode }).dgNode),
                  ) +
                  "40"
                );
              }
              if (node.type === "volumeNode") {
                return "#f9731640";
              }
              if (node.type === "roleGroup") {
                return swarmRoleColor((node.data as unknown as RoleGroupData).role, theme.mode) + "30";
              }
              if (node.type === "nodeGroup") {
                const role = (node.data as unknown as SwarmNodeGroupData).role;
                return role ? swarmRoleColor(role, theme.mode) + "60" : theme.nodeGhostBorder + "60";
              }
              if (node.type === "nodeServiceCard") {
                return stackColor((node.data as unknown as NodeServiceCardData).stack);
              }
              return theme.nodeBorder;
            }}
          />
        )}
      </ReactFlow>
      </div>
      )}
      </div>

      <Suspense fallback={null}>
        {logWindows.windows.length > 0 && (
          <LogWindowLayer controller={logWindows} onOpenInfo={openContainerInfo} />
        )}
      </Suspense>
    </div>
  );
}
