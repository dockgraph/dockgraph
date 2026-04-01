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
import type { Theme } from "../theme";
import type { ReactNode } from "react";

// Shared header for network and volume detail panels.
function ResourceHeader({ name, subtitle, theme }: { name: string; subtitle?: string; theme: Theme }) {
  return (
    <>
      <div style={{ fontSize: 16, fontWeight: 600, color: theme.nodeText, marginBottom: 2, wordBreak: "break-all" as const }}>
        {name}
      </div>
      {subtitle && (
        <div style={{ fontSize: 11, color: theme.nodeSubtext, marginBottom: 6 }}>
          {subtitle}
        </div>
      )}
    </>
  );
}

// Header for ghost (not-running) container detail panels.
function GhostHeader({ node, theme }: { node: DGNode; theme: Theme }) {
  return (
    <>
      <div style={{ fontSize: 16, fontWeight: 600, color: theme.nodeText, marginBottom: 2, wordBreak: "break-all" as const }}>
        {node.name}
      </div>
      {node.image && (
        <div style={{ fontSize: 11, color: theme.nodeSubtext, marginBottom: 6, wordBreak: "break-all" as const }}>
          {node.image}
        </div>
      )}
      <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
        <span style={{ width: 8, height: 8, borderRadius: "50%", background: "#64748b" }} />
        <span style={{ fontSize: 12, color: theme.panelText }}>Not Running</span>
        {node.source && (
          <span style={{ fontSize: 10, color: theme.nodeSubtext }}>from {node.source}</span>
        )}
      </div>
    </>
  );
}

function ContainerList({ containers, onNavigate, theme }: { containers: DGNode[]; onNavigate: (id: string) => void; theme: Theme }) {
  if (containers.length === 0) return null;
  return (
    <div>
      <div style={{ fontSize: 10, fontWeight: 600, color: theme.nodeSubtext, textTransform: "uppercase", letterSpacing: "0.5px", marginBottom: 6 }}>
        Containers ({containers.length})
      </div>
      {containers.map((c) => (
        <div
          key={c.id}
          onClick={() => onNavigate(`container:${c.name}`)}
          style={{
            fontSize: 11,
            color: theme.panelText,
            padding: "4px 0",
            cursor: "pointer",
            textDecoration: "underline",
            textDecorationColor: theme.panelBorder,
            textUnderlineOffset: 2,
          }}
        >
          {c.name}
        </div>
      ))}
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

  // Resolve a cross-reference name (from Docker inspect data) to an actual
  // graph node ID. Docker might return short names (e.g. service name without
  // compose prefix), so we try exact match first, then suffix match.
  const handleNavigate = useCallback(
    (targetId: string) => {
      // Exact match — fast path.
      if (dgNodes.some((n) => n.id === targetId)) {
        handleInfoClick(targetId);
        return;
      }

      // Parse type:name from the target.
      const sepIdx = targetId.indexOf(":");
      if (sepIdx < 0) return;
      const type = targetId.slice(0, sepIdx) as DGNode["type"];
      const name = targetId.slice(sepIdx + 1);

      // Suffix match: "elasticsearch-1" matches "demo-large-elasticsearch-1".
      const match = dgNodes.find(
        (n) =>
          n.type === type &&
          (n.name === name ||
            n.name.endsWith(`-${name}`) ||
            n.name.endsWith(`_${name}`)),
      );

      handleInfoClick(match ? match.id : targetId);
    },
    [dgNodes, handleInfoClick],
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
  const isGroupDetail = detailNodeId?.startsWith("group:") ?? false;
  const detailDgNode = detailNodeId
    ? dgNodes.find((n) => n.id === detailNodeId)
    : null;

  // Ghost detection — resources defined in compose but not yet created in Docker.
  const isGhostResource = detailDgNode?.status === "not_running";

  // Containers belonging to the selected network/group.
  const groupContainers = useMemo(() => {
    if (isGroupDetail) {
      return dgNodes.filter((n) => n.type === "container" && !n.source && !n.networkId);
    }
    if (isNetworkDetail && isGhostResource && detailNodeId) {
      return dgNodes.filter((n) => n.type === "container" && n.networkId === detailNodeId);
    }
    return [];
  }, [isGroupDetail, isNetworkDetail, isGhostResource, detailNodeId, dgNodes]);

  // Container detail derivation.
  const containerName = detailNodeId?.startsWith("container:")
    ? detailNodeId.replace("container:", "")
    : null;
  const isGhostDetail = !isVolumeDetail && !isNetworkDetail && isGhostResource;
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
  const isGhostVolume = isVolumeDetail && isGhostResource;
  const {
    data: volumeDetailData,
    loading: volumeDetailLoading,
    error: volumeDetailError,
  } = useVolumeDetail(isGhostVolume ? null : volumeName);

  // Network detail derivation.
  const networkName = isNetworkDetail
    ? (detailNodeId?.replace("network:", "") ?? null)
    : null;
  const isGhostNetwork = isNetworkDetail && isGhostResource;
  const {
    data: networkDetailData,
    loading: networkDetailLoading,
    error: networkDetailError,
  } = useNetworkDetail(isGhostNetwork ? null : networkName);

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
          isGroupDetail || isGhostResource
            ? false
            : isNetworkDetail
              ? networkDetailLoading
              : isVolumeDetail
                ? volumeDetailLoading
                : detailLoading
        }
        error={
          isGroupDetail || isGhostResource
            ? null
            : isNetworkDetail
              ? networkDetailError
              : isVolumeDetail
                ? volumeDetailError
                : detailError
        }
        header={
          isGroupDetail ? (
            <ResourceHeader name="Unmanaged" subtitle="Default bridge network" theme={theme} />
          ) : isGhostResource && detailDgNode ? (
            <GhostHeader node={detailDgNode} theme={theme} />
          ) : (isNetworkDetail || isVolumeDetail) && detailDgNode ? (
            <ResourceHeader name={detailDgNode.name} subtitle={detailDgNode.driver} theme={theme} />
          ) : detailData ? (
            <DetailPanelHeader detail={detailData} />
          ) : null
        }
      >
        {isGroupDetail ? (
          <div style={{ padding: "0 16px 12px" }}>
            <div style={{ fontSize: 11, color: theme.nodeSubtext, marginBottom: 10 }}>
              Containers not assigned to a named network.
            </div>
            <ContainerList containers={groupContainers} onNavigate={handleNavigate} theme={theme} />
          </div>
        ) : isGhostResource && detailDgNode ? (
          <div style={{ padding: "0 16px 12px" }}>
            {detailDgNode.compose && (
              <DetailPanelCompose
                compose={detailDgNode.compose}
                image={detailDgNode.image}
              />
            )}
            {detailDgNode.source && (
              <div style={{ fontSize: 11, color: theme.nodeSubtext, marginBottom: 10 }}>
                Defined in {detailDgNode.source}
              </div>
            )}
            <ContainerList containers={groupContainers} onNavigate={handleNavigate} theme={theme} />
          </div>
        ) : isNetworkDetail && networkDetailData ? (
          <DetailPanelNetworkInfo network={networkDetailData} onNavigate={handleNavigate} />
        ) : isVolumeDetail && volumeDetailData ? (
          <DetailPanelVolume volume={volumeDetailData} />
        ) : detailData ? (
          <>
            <DetailPanelStats
              stats={containerName ? statsMap.get(detailData.name) : undefined}
            />
            <DetailPanelProcess detail={detailData} />
            <DetailPanelPorts ports={detailData.ports} />
            <DetailPanelMounts mounts={detailData.mounts} onNavigate={handleNavigate} />
            <DetailPanelNetwork
              networkMode={detailData.networkMode}
              networks={detailData.networks}
              onNavigate={handleNavigate}
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
