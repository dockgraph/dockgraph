import { useCallback, useEffect, useMemo, useState } from 'react';
import { useReactFlow } from '@xyflow/react';
import { useContainerDetail } from './useContainerDetail';
import { useVolumeDetail } from './useVolumeDetail';
import { useNetworkDetail } from './useNetworkDetail';
import { useServiceDetail } from './useServiceDetail';
import { projectOf, resolveNodeRef } from '../utils/stack';
import { canvasNodesShowing, isRoleGroupId, roleOfGroupId } from '../utils/nodeTransform';
import type { DGNode, DGEdge, VolumeMount, SwarmRole } from '../types';

/** Discriminated union describing the resolved detail panel variant. */
export type DetailVariant =
  | { kind: 'none' }
  | { kind: 'container'; containerName: string }
  | { kind: 'service'; serviceName: string }
  | { kind: 'volume'; volumeName: string }
  | { kind: 'network'; networkName: string; owner?: string }
  | { kind: 'group' }
  | { kind: 'swarmnode'; node: DGNode }
  | { kind: 'rolegroup'; role: SwarmRole }
  | { kind: 'ghost-container'; node: DGNode }
  | { kind: 'ghost-service'; node: DGNode }
  | { kind: 'ghost-volume'; node: DGNode }
  | { kind: 'ghost-network'; node: DGNode };

/**
 * Manages all detail panel state: which resource is selected, type detection,
 * ghost detection, cross-reference navigation, and data fetching.
 *
 * `onOpen` is told the canvas node every panel is opened for (from a node's
 * ☰, a table row, the dashboard or a link inside a panel), so the canvas can
 * highlight it the same way whichever way it was opened.
 */
export function useDetailPanel(dgNodes: DGNode[], dgEdges: DGEdge[], onOpen?: (canvasId: string) => void) {
  const { fitView, getNodes } = useReactFlow();
  const [detailNodeId, setDetailNodeId] = useState<string | null>(null);
  // Canvas node the viewport centres on. Usually the opened resource's own
  // node; a service opened from one of its per-node cards centres on that
  // card, as the service has one card per swarm node and no node of its own.
  const [focusNodeId, setFocusNodeId] = useState<string | null>(null);

  const handleInfoClick = useCallback((nodeId: string, focusId: string = nodeId) => {
    setDetailNodeId(nodeId);
    setFocusNodeId(focusId);
    onOpen?.(focusId);
  }, [onOpen]);

  const closeDetail = useCallback(() => {
    setDetailNodeId(null);
    setFocusNodeId(null);
  }, []);

  // Fit the view to the focused node after the canvas resizes for the
  // detail panel. A resource drawn as several nodes (a service's cards) is
  // fitted as a whole, and one not on the canvas leaves the view alone:
  // fitting unknown ids would zoom onto the canvas origin.
  useEffect(() => {
    if (!focusNodeId) return;
    const timer = setTimeout(() => {
      const ids = canvasNodesShowing(getNodes(), focusNodeId);
      if (ids.length === 0) return;
      fitView({
        nodes: ids.map((id) => ({ id })),
        duration: 300,
        maxZoom: 1.5,
        padding: 0.3,
      });
    }, 50);
    return () => clearTimeout(timer);
  }, [focusNodeId, fitView, getNodes]);

  // Resolve cross-reference names to graph node IDs. Supports suffix matching
  // for Docker's short names (a compose service name without its project
  // prefix, `{stack}_{svc}` swarm services, task container names), preferring
  // matches in the stack of the resource currently shown. A service linked
  // from a swarm node's panel centres on its card on that node, not on every
  // node it runs on.
  const handleNavigate = useCallback(
    (targetId: string) => {
      if (targetId.indexOf(':') < 0 && !dgNodes.some((n) => n.id === targetId)) return;
      const from = detailNodeId ? dgNodes.find((n) => n.id === detailNodeId) : undefined;
      const target = resolveNodeRef(dgNodes, targetId, from ? projectOf(from) : undefined);
      const near = detailNodeId ? canvasNodesShowing(getNodes(), target, detailNodeId) : [];
      handleInfoClick(target, near.length === 1 ? near[0] : target);
    },
    [dgNodes, detailNodeId, getNodes, handleInfoClick],
  );

  // Resolve the detail node type and ghost status.
  const detailOpen = detailNodeId !== null;
  const isVolumeDetail = detailNodeId?.startsWith('volume:') ?? false;
  const isNetworkDetail = detailNodeId?.startsWith('network:') ?? false;
  const isGroupDetail = detailNodeId?.startsWith('group:') ?? false;
  const isServiceDetail = detailNodeId?.startsWith('service:') ?? false;
  const isSwarmNodeDetail = detailNodeId?.startsWith('swarmnode:') ?? false;
  const detailDgNode = detailNodeId
    ? dgNodes.find((n) => n.id === detailNodeId)
    : null;
  const isGhostResource = detailDgNode?.status === 'not_running';

  // Resolve the active variant.
  const variant: DetailVariant = useMemo(() => {
    if (!detailNodeId) return { kind: 'none' };
    if (isGroupDetail) return { kind: 'group' };
    // Swarm nodes and their role groups need no fetch: the graph carries
    // everything shown (the managers' quorum is asked for by the panel).
    if (isSwarmNodeDetail) return detailDgNode ? { kind: 'swarmnode', node: detailDgNode } : { kind: 'none' };
    if (isRoleGroupId(detailNodeId)) {
      const role = roleOfGroupId(detailNodeId);
      return role ? { kind: 'rolegroup', role } : { kind: 'none' };
    }
    if (isGhostResource && detailDgNode) {
      if (isVolumeDetail) return { kind: 'ghost-volume', node: detailDgNode };
      if (isNetworkDetail) return { kind: 'ghost-network', node: detailDgNode };
      if (isServiceDetail) return { kind: 'ghost-service', node: detailDgNode };
      return { kind: 'ghost-container', node: detailDgNode };
    }
    if (isNetworkDetail) return { kind: 'network', networkName: detailNodeId.replace('network:', ''), owner: detailDgNode?.owner };
    if (isVolumeDetail) return { kind: 'volume', volumeName: detailNodeId.replace('volume:', '') };
    if (isServiceDetail) return { kind: 'service', serviceName: detailNodeId.replace('service:', '') };
    return { kind: 'container', containerName: detailNodeId.replace('container:', '') };
  }, [detailNodeId, isGroupDetail, isSwarmNodeDetail, isGhostResource, isVolumeDetail, isNetworkDetail, isServiceDetail, detailDgNode]);

  // Containers belonging to the selected network/group.
  const groupContainers = useMemo(() => {
    if (isGroupDetail) {
      return dgNodes.filter((n) => n.type === 'container' && !n.source && !n.networkId);
    }
    if (isNetworkDetail && detailNodeId) {
      const ids = new Set(
        dgNodes
          .filter((n) => (n.type === 'container' || n.type === 'service') && n.networkId === detailNodeId)
          .map((n) => n.id),
      );
      for (const e of dgEdges) {
        if (e.type === 'secondary_network' && e.target === detailNodeId) {
          ids.add(e.source);
        }
      }
      return dgNodes.filter((n) => ids.has(n.id));
    }
    return [];
  }, [isGroupDetail, isNetworkDetail, detailNodeId, dgNodes, dgEdges]);

  // Containers using the selected volume.
  const volumeMounts = useMemo<VolumeMount[]>(() => {
    if (!isVolumeDetail || !detailNodeId) return [];
    const mounts: VolumeMount[] = [];
    for (const e of dgEdges) {
      if (e.type === 'volume_mount' && e.source === detailNodeId) {
        const node = dgNodes.find((n) => n.id === e.target);
        if (node) mounts.push({ node, mountPath: e.mountPath ?? '' });
      }
    }
    return mounts;
  }, [isVolumeDetail, detailNodeId, dgEdges, dgNodes]);

  // Resource detail fetching — only the relevant hook fetches based on variant.
  const containerName = variant.kind === 'container' ? variant.containerName : null;
  const volumeName = variant.kind === 'volume' ? variant.volumeName : null;
  const networkName = variant.kind === 'network' ? variant.networkName : null;
  const networkOwner = variant.kind === 'network' ? variant.owner : undefined;
  const serviceName = variant.kind === 'service' ? variant.serviceName : null;

  const { data: containerData, loading: containerLoading, error: containerError } = useContainerDetail(containerName);
  const { data: volumeData, loading: volumeLoading, error: volumeError } = useVolumeDetail(volumeName);
  const { data: networkData, loading: networkLoading, error: networkError } = useNetworkDetail(networkName, networkOwner);
  const { data: serviceData, loading: serviceLoading, error: serviceError } = useServiceDetail(serviceName);

  // Coalesce loading/error for the active variant.
  const loading = variant.kind === 'container' ? containerLoading
    : variant.kind === 'volume' ? volumeLoading
    : variant.kind === 'network' ? networkLoading
    : variant.kind === 'service' ? serviceLoading
    : false;

  const error = variant.kind === 'container' ? containerError
    : variant.kind === 'volume' ? volumeError
    : variant.kind === 'network' ? networkError
    : variant.kind === 'service' ? serviceError
    : null;

  return {
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
    loading,
    error,
    handleInfoClick,
    handleNavigate,
    closeDetail,
  };
}
