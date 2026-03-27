export interface PortMapping {
  host: number;
  container: number;
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
  source?: string;
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
  nodesUpdated?: Partial<DGNode>[];
  edgesAdded?: DGEdge[];
  edgesRemoved?: string[];
}

export interface WireMessage {
  type: 'snapshot' | 'delta';
  version: number;
  data: GraphSnapshot | DeltaUpdate;
}

// Node data types used by React Flow custom components

export interface ContainerNodeData {
  dgNode: DGNode;
  nodeWidth?: number;
}

export interface VolumeNodeData {
  dgNode: DGNode;
  nodeWidth?: number;
}

export interface NetworkGroupData {
  dgNode: DGNode;
}

/** Typed payload for ELK-routed edges. */
export interface ElkEdgeData {
  path?: string;
  edgeType?: string;
  active?: boolean;
  animated?: boolean;
  nodeCount?: number;
}
