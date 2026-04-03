import { useCallback, useEffect, useMemo, useState } from 'react';
import { useReactFlow } from '@xyflow/react';
import { useContainerDetail } from './useContainerDetail';
import { useVolumeDetail } from './useVolumeDetail';
import { useNetworkDetail } from './useNetworkDetail';
import type { DGNode, DGEdge, VolumeMount } from '../types';

/** Discriminated union describing the resolved detail panel variant. */
export type DetailVariant =
  | { kind: 'none' }
  | { kind: 'container'; containerName: string }
  | { kind: 'volume'; volumeName: string }
  | { kind: 'network'; networkName: string }
  | { kind: 'group' }
  | { kind: 'ghost-container'; node: DGNode }
  | { kind: 'ghost-volume'; node: DGNode }
  | { kind: 'ghost-network'; node: DGNode };

/**
 * Manages all detail panel state: which resource is selected, type detection,
 * ghost detection, cross-reference navigation, and data fetching.
 */
export function useDetailPanel(dgNodes: DGNode[], dgEdges: DGEdge[]) {
  const { fitView } = useReactFlow();
  const [detailNodeId, setDetailNodeId] = useState<string | null>(null);

  const handleInfoClick = useCallback(
    (nodeId: string) => setDetailNodeId(nodeId),
    [],
  );

  const closeDetail = useCallback(() => setDetailNodeId(null), []);

  // Fit view to the selected node after canvas resizes for the detail panel.
  useEffect(() => {
    if (!detailNodeId) return;
    const timer = setTimeout(() => {
      fitView({
        nodes: [{ id: detailNodeId }],
        duration: 300,
        maxZoom: 1.5,
        padding: 0.3,
      });
    }, 50);
    return () => clearTimeout(timer);
  }, [detailNodeId, fitView]);

  // Resolve cross-reference names to graph node IDs. Supports suffix matching
  // for Docker's short names (e.g. service name without compose prefix).
  const handleNavigate = useCallback(
    (targetId: string) => {
      if (dgNodes.some((n) => n.id === targetId)) {
        setDetailNodeId(targetId);
        return;
      }
      const sepIdx = targetId.indexOf(':');
      if (sepIdx < 0) return;
      const type = targetId.slice(0, sepIdx) as DGNode['type'];
      const name = targetId.slice(sepIdx + 1);
      const match = dgNodes.find(
        (n) =>
          n.type === type &&
          (n.name === name ||
            n.name.endsWith(`-${name}`) ||
            n.name.endsWith(`_${name}`)),
      );
      setDetailNodeId(match ? match.id : targetId);
    },
    [dgNodes],
  );

  // Resolve the detail node type and ghost status.
  const detailOpen = detailNodeId !== null;
  const isVolumeDetail = detailNodeId?.startsWith('volume:') ?? false;
  const isNetworkDetail = detailNodeId?.startsWith('network:') ?? false;
  const isGroupDetail = detailNodeId?.startsWith('group:') ?? false;
  const detailDgNode = detailNodeId
    ? dgNodes.find((n) => n.id === detailNodeId)
    : null;
  const isGhostResource = detailDgNode?.status === 'not_running';

  // Resolve the active variant.
  const variant: DetailVariant = useMemo(() => {
    if (!detailNodeId) return { kind: 'none' };
    if (isGroupDetail) return { kind: 'group' };
    if (isGhostResource && detailDgNode) {
      if (isVolumeDetail) return { kind: 'ghost-volume', node: detailDgNode };
      if (isNetworkDetail) return { kind: 'ghost-network', node: detailDgNode };
      return { kind: 'ghost-container', node: detailDgNode };
    }
    if (isNetworkDetail) return { kind: 'network', networkName: detailNodeId.replace('network:', '') };
    if (isVolumeDetail) return { kind: 'volume', volumeName: detailNodeId.replace('volume:', '') };
    return { kind: 'container', containerName: detailNodeId.replace('container:', '') };
  }, [detailNodeId, isGroupDetail, isGhostResource, isVolumeDetail, isNetworkDetail, detailDgNode]);

  // Containers belonging to the selected network/group.
  const groupContainers = useMemo(() => {
    if (isGroupDetail) {
      return dgNodes.filter((n) => n.type === 'container' && !n.source && !n.networkId);
    }
    if (isNetworkDetail && detailNodeId) {
      const ids = new Set(
        dgNodes.filter((n) => n.type === 'container' && n.networkId === detailNodeId).map((n) => n.id),
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

  const { data: containerData, loading: containerLoading, error: containerError } = useContainerDetail(containerName);
  const { data: volumeData, loading: volumeLoading, error: volumeError } = useVolumeDetail(volumeName);
  const { data: networkData, loading: networkLoading, error: networkError } = useNetworkDetail(networkName);

  // Coalesce loading/error for the active variant.
  const loading = variant.kind === 'container' ? containerLoading
    : variant.kind === 'volume' ? volumeLoading
    : variant.kind === 'network' ? networkLoading
    : false;

  const error = variant.kind === 'container' ? containerError
    : variant.kind === 'volume' ? volumeError
    : variant.kind === 'network' ? networkError
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
    loading,
    error,
    handleInfoClick,
    handleNavigate,
    closeDetail,
  };
}
