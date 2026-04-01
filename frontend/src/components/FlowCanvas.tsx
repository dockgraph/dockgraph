import { useCallback, useEffect, useMemo, useRef, useState } from "react";
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
import { ANIMATION_NODE_LIMIT } from "../utils/constants";

import { ContainerNode } from "./ContainerNode";
import { NetworkGroup } from "./NetworkGroup";
import { VolumeNode } from "./VolumeNode";
import { ElkEdge } from "./ElkEdge";
import { CanvasEdgeLayer, type CanvasEdgeLayerHandle } from "./CanvasEdgeLayer";
import { ThemeToggle } from "./ThemeToggle";
import { LogoutButton } from "./LogoutButton";
import { StatusIndicator } from "./StatusIndicator";
import { SearchFilter } from "./SearchFilter";
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
import { DetailPanelCompose } from "./panels/DetailPanelCompose";
import { DetailPanelVolume } from "./panels/DetailPanelVolume";
import { DetailPanelNetworkInfo } from "./panels/DetailPanelNetworkInfo";
import { useGraphLayout } from "../hooks/useGraphLayout";
import { useSelectionHighlight } from "../hooks/useSelectionHighlight";
import { useContainerDetail } from "../hooks/useContainerDetail";
import { useVolumeDetail } from "../hooks/useVolumeDetail";
import { useNetworkDetail } from "../hooks/useNetworkDetail";
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
  const { fitView } = useReactFlow();
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

  // Detail panel state.
  const [detailNodeId, setDetailNodeId] = useState<string | null>(null);

  const handleInfoClick = useCallback(
    (nodeId: string) => {
      setDetailNodeId(nodeId);
      selectNodeRef.current?.(nodeId);
      fitView({
        nodes: [{ id: nodeId }],
        duration: 300,
        maxZoom: 1.5,
        padding: 0.3,
      });
    },
    [fitView],
  );

  // Inject live stats and info callback into container and volume node data (doesn't affect layout).
  const enrichedNodes = useMemo(() => {
    return nodes.map((n) => {
      if (n.type === "containerNode") {
        const dgNode = (n.data as { dgNode: DGNode }).dgNode;
        const s = statsMap.get(dgNode.name);
        return {
          ...n,
          data: { ...n.data, stats: s, onInfoClick: handleInfoClick },
        };
      }
      if (n.type === "volumeNode") {
        return { ...n, data: { ...n.data, onInfoClick: handleInfoClick } };
      }
      if (n.type === "networkGroup") {
        return { ...n, data: { ...n.data, onInfoClick: handleInfoClick } };
      }
      return n;
    });
  }, [nodes, statsMap, handleInfoClick]);

  const detailOpen = detailNodeId !== null;
  const isVolumeDetail = detailNodeId?.startsWith("volume:") ?? false;
  const isNetworkDetail = detailNodeId?.startsWith("network:") ?? false;
  const detailDgNode = detailNodeId
    ? dgNodes.find((n) => n.id === detailNodeId)
    : null;

  // Container detail derivation.
  const containerName = !isVolumeDetail && !isNetworkDetail
    ? (detailNodeId?.replace("container:", "") ?? null)
    : null;
  const isGhostDetail =
    !isVolumeDetail && !isNetworkDetail && detailDgNode?.status === "not_running";
  const {
    data: detailData,
    loading: detailLoading,
    error: detailError,
  } = useContainerDetail(
    isGhostDetail || isVolumeDetail || isNetworkDetail ? null : containerName,
  );

  // Volume detail derivation.
  const volumeName = isVolumeDetail
    ? (detailNodeId?.replace("volume:", "") ?? null)
    : null;
  const {
    data: volumeDetailData,
    loading: volumeDetailLoading,
    error: volumeDetailError,
  } = useVolumeDetail(isVolumeDetail ? volumeName : null);

  // Network detail derivation.
  const networkName = isNetworkDetail
    ? (detailNodeId?.replace("network:", "") ?? null)
    : null;
  const {
    data: networkDetailData,
    loading: networkDetailLoading,
    error: networkDetailError,
  } = useNetworkDetail(isNetworkDetail ? networkName : null);

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
  selectNodeRef.current = selectNode;

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
  });

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

  const closeDetail = useCallback(() => setDetailNodeId(null), []);

  return (
    <div
      style={{
        width: "100%",
        height: "100%",
        position: "relative",
        overflow: "hidden",
      }}
    >
      <style>{'.react-flow__edge { cursor: default; }'}</style>
      <StatusIndicator connected={connected} />
      <LogoutButton />
      <SearchFilter search={search} />

      <DetailPanel
        open={detailOpen}
        onClose={closeDetail}
        loading={
          isNetworkDetail
            ? networkDetailLoading
            : isVolumeDetail
              ? volumeDetailLoading
              : !isGhostDetail && detailLoading
        }
        error={
          isNetworkDetail
            ? networkDetailError
            : isVolumeDetail
              ? volumeDetailError
              : !isGhostDetail
                ? detailError
                : null
        }
        header={
          isNetworkDetail && detailDgNode ? (
            <>
              <div
                style={{
                  fontSize: 16,
                  fontWeight: 600,
                  color: theme.nodeText,
                  marginBottom: 2,
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                  whiteSpace: "nowrap",
                }}
                title={detailDgNode.name}
              >
                {detailDgNode.name}
              </div>
              {detailDgNode.driver && (
                <div
                  style={{
                    fontSize: 11,
                    color: theme.nodeSubtext,
                    marginBottom: 6,
                    overflow: "hidden",
                    textOverflow: "ellipsis",
                    whiteSpace: "nowrap",
                  }}
                >
                  {detailDgNode.driver}
                </div>
              )}
            </>
          ) : isVolumeDetail && detailDgNode ? (
            <>
              <div
                style={{
                  fontSize: 16,
                  fontWeight: 600,
                  color: theme.nodeText,
                  marginBottom: 2,
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                  whiteSpace: "nowrap",
                }}
                title={detailDgNode.name}
              >
                {detailDgNode.name}
              </div>
              {detailDgNode.driver && (
                <div
                  style={{
                    fontSize: 11,
                    color: theme.nodeSubtext,
                    marginBottom: 6,
                    overflow: "hidden",
                    textOverflow: "ellipsis",
                    whiteSpace: "nowrap",
                  }}
                >
                  {detailDgNode.driver}
                </div>
              )}
            </>
          ) : isGhostDetail && detailDgNode ? (
            <>
              <div
                style={{
                  fontSize: 16,
                  fontWeight: 600,
                  color: theme.nodeText,
                  marginBottom: 2,
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                  whiteSpace: "nowrap",
                }}
                title={detailDgNode.name}
              >
                {detailDgNode.name}
              </div>
              {detailDgNode.image && (
                <div
                  style={{
                    fontSize: 11,
                    color: theme.nodeSubtext,
                    marginBottom: 6,
                    overflow: "hidden",
                    textOverflow: "ellipsis",
                    whiteSpace: "nowrap",
                  }}
                  title={detailDgNode.image}
                >
                  {detailDgNode.image}
                </div>
              )}
              <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
                <span
                  style={{
                    width: 8,
                    height: 8,
                    borderRadius: "50%",
                    background: "#64748b",
                  }}
                />
                <span style={{ fontSize: 12, color: theme.panelText }}>
                  Not Running
                </span>
                {detailDgNode.source && (
                  <span style={{ fontSize: 10, color: theme.nodeSubtext }}>
                    from {detailDgNode.source}
                  </span>
                )}
              </div>
            </>
          ) : detailData ? (
            <DetailPanelHeader detail={detailData} />
          ) : null
        }
      >
        {isNetworkDetail && networkDetailData ? (
          <DetailPanelNetworkInfo network={networkDetailData} />
        ) : isVolumeDetail && volumeDetailData ? (
          <DetailPanelVolume volume={volumeDetailData} />
        ) : isGhostDetail && detailDgNode ? (
          <>
            {detailDgNode.compose && (
              <DetailPanelCompose
                compose={detailDgNode.compose}
                image={detailDgNode.image}
              />
            )}
          </>
        ) : detailData ? (
          <>
            <DetailPanelStats
              stats={containerName ? statsMap.get(detailData.name) : undefined}
            />
            <DetailPanelProcess detail={detailData} />
            <DetailPanelPorts ports={detailData.ports} />
            <DetailPanelMounts mounts={detailData.mounts} />
            <DetailPanelNetwork
              networkMode={detailData.networkMode}
              networks={detailData.networks}
            />
            <DetailPanelSecurity security={detailData.security} />
            <DetailPanelEnv env={detailData.env} />
            <DetailPanelLabels labels={detailData.labels} />
            <DetailPanelHealth health={detailData.health} />
            <DetailPanelLogs containerId={containerName} active={detailOpen} />
          </>
        ) : null}
      </DetailPanel>

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
    </div>
  );
}
