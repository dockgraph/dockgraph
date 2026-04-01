import { useCallback, useMemo, useRef, useState } from 'react';
import {
  ReactFlow,
  MiniMap,
  Panel,
  Controls,
  Background,
  BackgroundVariant,
  type Node as RFNode,
  type Edge as RFEdge,
} from '@xyflow/react';
import { ANIMATION_NODE_LIMIT } from '../utils/constants';

import { ContainerNode } from './ContainerNode';
import { NetworkGroup } from './NetworkGroup';
import { VolumeNode } from './VolumeNode';
import { ElkEdge } from './ElkEdge';
import { CanvasEdgeLayer, type CanvasEdgeLayerHandle } from './CanvasEdgeLayer';
import { ThemeToggle } from './ThemeToggle';
import { LogoutButton } from './LogoutButton';
import { StatusIndicator } from './StatusIndicator';
import { SearchFilter } from './SearchFilter';
import { DetailPanel } from './panels/DetailPanel';
import { DetailPanelHeader } from './panels/DetailPanelHeader';
import { DetailPanelStats } from './panels/DetailPanelStats';
import { DetailPanelProcess } from './panels/DetailPanelProcess';
import { DetailPanelPorts } from './panels/DetailPanelPorts';
import { DetailPanelMounts } from './panels/DetailPanelMounts';
import { DetailPanelEnv } from './panels/DetailPanelEnv';
import { DetailPanelLabels } from './panels/DetailPanelLabels';
import { DetailPanelNetwork } from './panels/DetailPanelNetwork';
import { DetailPanelSecurity } from './panels/DetailPanelSecurity';
import { DetailPanelHealth } from './panels/DetailPanelHealth';
import { DetailPanelLogs } from './panels/DetailPanelLogs';
import { useGraphLayout } from '../hooks/useGraphLayout';
import { useSelectionHighlight } from '../hooks/useSelectionHighlight';
import { useContainerDetail } from '../hooks/useContainerDetail';
import { useSearchFilter } from '../hooks/useSearchFilter';
import { networkColor } from '../utils/colors';
import { useTheme } from '../theme';
import type { DGNode, DGEdge, ContainerStatsData } from '../types';
import type { ReactNode } from 'react';

function Overlay({ children }: { children: ReactNode }) {
  return (
    <div
      role="status"
      aria-live="polite"
      style={{
        position: 'absolute',
        inset: 0,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 5,
        pointerEvents: 'none',
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

export function FlowCanvas({ dgNodes, dgEdges, connected, ready, statsMap }: FlowCanvasProps) {
  const { theme } = useTheme();
  const canvasEdgeRef = useRef<CanvasEdgeLayerHandle>(null);

  const { nodes, edges, onNodesChange, onEdgesChange, layoutBusy, layoutError } =
    useGraphLayout(dgNodes, dgEdges, theme.edgeStroke);

  const hasVisibleNodes = dgNodes.some((n) => n.type === 'container' || n.type === 'volume');
  const showEmptyState = !ready || !hasVisibleNodes;
  const largeGraph = dgNodes.length > ANIMATION_NODE_LIMIT;

  // Inject live stats into container node data (doesn't affect layout).
  const nodesWithStats = useMemo(() => {
    if (statsMap.size === 0) return nodes;
    return nodes.map((n) => {
      if (n.type !== 'containerNode') return n;
      const dgNode = (n.data as { dgNode: DGNode }).dgNode;
      const s = statsMap.get(dgNode.name);
      if (!s) return n;
      return { ...n, data: { ...n.data, stats: s } };
    });
  }, [nodes, statsMap]);

  // Detail panel state.
  const [detailContainerId, setDetailContainerId] = useState<string | null>(null);
  const detailOpen = detailContainerId !== null;
  const containerName = detailContainerId?.replace('container:', '') ?? null;
  const { data: detailData, loading: detailLoading, error: detailError } = useContainerDetail(containerName);

  // Search & filter.
  const search = useSearchFilter(dgNodes);

  const { styledNodes, styledEdges, canvasEdges, svgEdges, onNodeClick, onEdgeClick, onPaneClick } =
    useSelectionHighlight(nodesWithStats, edges, largeGraph, search.matchingNodeIds);

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

  // Double-click opens the detail panel for containers.
  const onNodeDoubleClick = useCallback((_: React.MouseEvent, node: RFNode) => {
    if (node.type === 'containerNode') {
      setDetailContainerId(node.id);
    }
  }, []);

  const closeDetail = useCallback(() => setDetailContainerId(null), []);

  return (
    <div style={{ width: '100%', height: '100%', position: 'relative' }}>
      <StatusIndicator connected={connected} />
      <LogoutButton />
      <SearchFilter search={search} />

      <DetailPanel
        open={detailOpen}
        onClose={closeDetail}
        loading={detailLoading}
        error={detailError}
      >
        {detailData && (
          <>
            <DetailPanelHeader detail={detailData} />
            <DetailPanelStats stats={containerName ? statsMap.get(detailData.name) : undefined} />
            <DetailPanelProcess detail={detailData} />
            <DetailPanelPorts ports={detailData.ports} />
            <DetailPanelMounts mounts={detailData.mounts} />
            <DetailPanelNetwork networkMode={detailData.networkMode} networks={detailData.networks} />
            <DetailPanelSecurity security={detailData.security} />
            <DetailPanelEnv env={detailData.env} />
            <DetailPanelLabels labels={detailData.labels} />
            <DetailPanelHealth health={detailData.health} />
            <DetailPanelLogs containerId={containerName} active={detailOpen} />
          </>
        )}
      </DetailPanel>

      {showEmptyState && (
        <Overlay>
          <p style={{ color: theme.nodeSubtext, fontSize: 14 }}>
            {!ready
              ? 'Connecting to backend\u2026'
              : 'No containers detected. Start a container to visualize the graph.'}
          </p>
        </Overlay>
      )}

      {layoutBusy && !showEmptyState && (
        <Overlay>
          <p style={{ color: theme.nodeSubtext, fontSize: 14 }}>
            Computing layout for {dgNodes.length} nodes\u2026
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
        onNodeDoubleClick={onNodeDoubleClick}
        onEdgeClick={onEdgeClick}
        onPaneClick={handlePaneClick}
        nodeTypes={nodeTypes}
        edgeTypes={edgeTypes}
        nodesDraggable={false}
        nodesConnectable={false}
        elevateNodesOnSelect={false}
        onlyRenderVisibleElements
        fitView
        minZoom={0.05}
        maxZoom={2}
        style={{ background: theme.canvasBg }}
      >
        {!largeGraph && <Background variant={BackgroundVariant.Dots} color={theme.dotColor} gap={20} />}
        <Panel position="bottom-left" style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 10, margin: 15 }}>
          <Controls
            showInteractive={false}
            position="bottom-left"
            style={{ position: 'relative', background: theme.panelBg, border: `1px solid ${theme.panelBorder}`, borderRadius: 6 }}
          />
          <ThemeToggle />
        </Panel>
        {!largeGraph && (
          <MiniMap
            style={{ background: theme.minimapBg, border: `1px solid ${theme.panelBorder}` }}
            maskColor={theme.minimapMask}
            nodeColor={(node) => {
              if (node.type === 'networkGroup') {
                return networkColor((node.data as { dgNode: DGNode }).dgNode.name) + '40';
              }
              if (node.type === 'volumeNode') {
                return '#f9731640';
              }
              return theme.nodeBorder;
            }}
          />
        )}
      </ReactFlow>
    </div>
  );
}
