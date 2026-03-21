export interface PortMapping {
  host: number;
  container: number;
}

export interface DFNode {
  id: string;
  type: 'container' | 'network' | 'volume';
  name: string;
  image?: string;
  status?: string;
  ports?: PortMapping[];
  labels?: Record<string, string>;
  networkId?: string;
  driver?: string;
  source?: string;
}

export interface DFEdge {
  id: string;
  type: 'volume_mount' | 'depends_on' | 'secondary_network';
  source: string;
  target: string;
  mountPath?: string;
}

export interface GraphSnapshot {
  nodes: DFNode[];
  edges: DFEdge[];
}

export interface DeltaUpdate {
  nodesAdded?: DFNode[];
  nodesRemoved?: string[];
  nodesUpdated?: Partial<DFNode>[];
  edgesAdded?: DFEdge[];
  edgesRemoved?: string[];
}

export interface WireMessage {
  type: 'snapshot' | 'delta';
  version: number;
  data: GraphSnapshot | DeltaUpdate;
}

// Node data types used by React Flow custom components

export interface ContainerNodeData {
  dfNode: DFNode;
  nodeWidth?: number;
}

export interface VolumeNodeData {
  dfNode: DFNode;
  nodeWidth?: number;
}

export interface NetworkGroupData {
  dfNode: DFNode;
}

/** Typed payload for ELK-routed edges. */
export interface ElkEdgeData {
  path?: string;
  edgeType?: string;
  active?: boolean;
}
