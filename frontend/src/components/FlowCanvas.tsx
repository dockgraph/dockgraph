import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  ReactFlow,
  MiniMap,
  Panel,
  Controls,
  Background,
  BackgroundVariant,
  type Node as RFNode,
  type Edge as RFEdge,
} from "@xyflow/react";
import { ANIMATION_NODE_LIMIT, DETAIL_PANEL_WIDTH } from "../utils/constants";

import { ContainerNode } from "./ContainerNode";
import { NetworkGroup } from "./NetworkGroup";
import { VolumeNode } from "./VolumeNode";
import { ElkEdge } from "./ElkEdge";
import { CanvasEdgeLayer, type CanvasEdgeLayerHandle } from "./CanvasEdgeLayer";
import { ThemeToggle } from "./ThemeToggle";
import { LogoutButton } from "./LogoutButton";
import { StatusIndicator } from "./StatusIndicator";
import { SearchFilter } from "./SearchFilter";
import { ViewTabs } from "./ViewTabs";
import type { ViewKey } from "./ViewTabs";
import { TableView } from "./table/TableView";
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
import { useGraphLayout } from "../hooks/useGraphLayout";
import { useSelectionHighlight } from "../hooks/useSelectionHighlight";
import { useDetailPanel } from "../hooks/useDetailPanel";
import { useSearchFilter } from "../hooks/useSearchFilter";
import { networkColor } from "../utils/colors";
import { useTheme } from "../theme";
import type { DGNode, DGEdge, ContainerStatsData } from "../types";
import type { ReactNode } from "react";

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
  networkGroup: NetworkGroup,
  volumeNode: VolumeNode,
};

const edgeTypes = {
  elk: ElkEdge,
};

interface FlowCanvasProps {
  dgNodes: DGNode[];
  dgEdges: DGEdge[];
  connected: boolean;
  ready: boolean;
  statsMap: Map<string, ContainerStatsData>;
}

export function FlowCanvas({
  dgNodes,
  dgEdges,
  connected,
  ready,
  statsMap,
}: FlowCanvasProps) {
  const { theme } = useTheme();
  const canvasEdgeRef = useRef<CanvasEdgeLayerHandle>(null);
  const selectNodeRef = useRef<((id: string) => void) | undefined>(undefined);

  const {
    nodes,
    edges,
    onNodesChange,
    onEdgesChange,
    layoutBusy,
    layoutError,
  } = useGraphLayout(dgNodes, dgEdges, theme.edgeStroke);

  const hasVisibleNodes = dgNodes.some(
    (n) => n.type === "container" || n.type === "volume",
  );
  const showEmptyState = !ready || !hasVisibleNodes;
  const largeGraph = dgNodes.length > ANIMATION_NODE_LIMIT;

  // View navigation state.
  const [activeView, setActiveView] = useState<ViewKey>("graph");

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
      selectNodeRef.current?.(nodeId);
    },
    [handleInfoClick],
  );

  // Inject live stats and info callback into container and volume node data (doesn't affect layout).
  const enrichedNodes = useMemo(() => {
    return nodes.map((n) => {
      if (n.type === "containerNode") {
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
      return n;
    });
  }, [nodes, statsMap, handleInfoClickWithSelect]);

  // Search & filter.
  const search = useSearchFilter(dgNodes);

  const {
    styledNodes,
    styledEdges,
    canvasEdges,
    svgEdges,
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
  );
  useEffect(() => { selectNodeRef.current = selectNode; }, [selectNode]);

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
          ) : variant.kind === 'ghost-container' || variant.kind === 'ghost-volume' || variant.kind === 'ghost-network' ? (
            <GhostHeader node={variant.node} theme={theme} />
          ) : (variant.kind === 'network' || variant.kind === 'volume') && detailDgNode ? (
            <ResourceHeader name={detailDgNode.name} subtitle={detailDgNode.driver} theme={theme} />
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
        ) : variant.kind === 'ghost-container' ? (
          <GhostContainerPanel node={variant.node} onNavigate={handleNavigate} />
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
            <DetailPanelLogs containerId={variant.kind === 'container' ? variant.containerName : null} active={detailOpen} />
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
          top: 10,
          left: 10,
          right: 10,
          zIndex: 10,
          display: "flex",
          alignItems: "flex-start",
          justifyContent: "space-between",
        }}
      >
        <ViewTabs activeView={activeView} onViewChange={setActiveView} />
        <div style={{ position: "absolute", left: "50%", transform: "translateX(-50%)" }}>
          <SearchFilter search={search} />
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
          <LogoutButton />
          <StatusIndicator connected={connected} />
        </div>
      </div>

      {activeView === "table" ? (
        <div style={{ position: "absolute", inset: 0, top: 50, display: "flex", flexDirection: "column" }}>
          <TableView
            nodes={dgNodes}
            edges={dgEdges}
            statsMap={statsMap}
            matchingNodeIds={search.matchingNodeIds}
            selectedNodeId={detailNodeId}
            onRowClick={handleInfoClickWithSelect}
          />
        </div>
      ) : activeView !== "graph" ? (
        <div
          style={{
            position: "absolute",
            inset: 0,
            top: 50,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            flexDirection: "column",
            gap: 12,
          }}
        >
          <p style={{ color: theme.nodeSubtext, fontSize: 16, fontWeight: 600, margin: 0 }}>
            Dashboard
          </p>
          <p style={{ color: theme.nodeSubtext, fontSize: 13, margin: 0, opacity: 0.6 }}>
            Resource metrics and insights — coming soon
          </p>
        </div>
      ) : (
      <>
      {showEmptyState && (
        <Overlay>
          <p style={{ color: theme.nodeSubtext, fontSize: 14 }}>
            {!ready
              ? "Connecting to backend..."
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
        edges={largeGraph ? svgEdges : styledEdges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        onNodeClick={largeGraph ? handleNodeClick : onNodeClick}
        onPaneClick={handlePaneClick}
        nodeTypes={nodeTypes}
        edgeTypes={edgeTypes}
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
          <Background
            variant={BackgroundVariant.Dots}
            color={theme.dotColor}
            gap={20}
          />
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
            style={{
              position: "relative",
              background: theme.panelBg,
              border: `1px solid ${theme.panelBorder}`,
              borderRadius: 6,
            }}
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
                  networkColor((node.data as { dgNode: DGNode }).dgNode.name) +
                  "40"
                );
              }
              if (node.type === "volumeNode") {
                return "#f9731640";
              }
              return theme.nodeBorder;
            }}
          />
        )}
      </ReactFlow>
      </>
      )}
      </div>
    </div>
  );
}
