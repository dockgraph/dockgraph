export interface PortMapping {
  host: number;
  container: number;
}

export interface ComposeConfig {
  service: string;
  command?: string[];
  entrypoint?: string[];
  environment?: Record<string, string>;
  restart?: string;
  dependsOn?: string[];
  volumes?: string[];
  networks?: string[];
  user?: string;
  workingDir?: string;
  privileged?: boolean;
  readOnly?: boolean;
  capAdd?: string[];
  capDrop?: string[];
}

export interface DGNode {
  id: string;
  type: 'container' | 'network' | 'volume';
  name: string;
  image?: string;
  status?: string;
  ports?: PortMapping[];
  labels?: Record<string, string>;
  networkId?: string;
  driver?: string;
  subnet?: string;
  gateway?: string;
  source?: string;
  compose?: ComposeConfig;
}

export interface DGEdge {
  id: string;
  type: 'volume_mount' | 'depends_on' | 'secondary_network';
  source: string;
  target: string;
  mountPath?: string;
}

export interface GraphSnapshot {
  nodes: DGNode[];
  edges: DGEdge[];
}

export interface DeltaUpdate {
  nodesAdded?: DGNode[];
  nodesRemoved?: string[];
  nodesUpdated?: DGNode[];
  edgesAdded?: DGEdge[];
  edgesRemoved?: string[];
}

export type WireMessage =
  | { type: 'snapshot'; version: number; data: GraphSnapshot }
  | { type: 'delta'; version: number; data: DeltaUpdate }
  | { type: 'auth_expired'; version: number }
  | { type: 'stats'; version: number; data: import('./types/stats').StatsMessage };

// Node data types used by React Flow custom components

export interface ContainerNodeData {
  dgNode: DGNode;
  nodeWidth?: number;
  stats?: import('./types/stats').ContainerStatsData;
  onInfoClick?: (containerId: string) => void;
}

export interface VolumeNodeData {
  dgNode: DGNode;
  nodeWidth?: number;
  onInfoClick?: (volumeId: string) => void;
}

export interface NetworkGroupData {
  dgNode: DGNode;
  onInfoClick?: (networkId: string) => void;
}

/** Typed payload for ELK-routed edges. */
export interface ElkEdgeData {
  path?: string;
  edgeType?: string;
  active?: boolean;
  animated?: boolean;
  nodeCount?: number;
}

export type { ContainerStatsData, StatsMessage, ContainerDetail, LogLine, VolumeDetail, NetworkDetail } from './types/stats';
