export interface PortMapping {
  host: number;
  container: number;
  protocol?: string;
}

export interface ComposeConfig {
  service: string;
  command?: string[];
  entrypoint?: string[];
  environment?: Record<string, string>;
  labels?: Record<string, string>;
  restart?: string;
  dependsOn?: string[];
  volumes?: import('./types/stats').Mount[];
  networks?: string[];
  user?: string;
  workingDir?: string;
  privileged?: boolean;
  readOnly?: boolean;
  capAdd?: string[];
  capDrop?: string[];
}

/** Number of running tasks versus the desired count for a swarm service. */
export interface ReplicaCount {
  running: number;
  desired: number;
}

/** A single swarm task of a service, placed on a cluster node. */
export interface TaskInfo {
  id: string;
  slot?: number;
  nodeId?: string;
  nodeHostname?: string;
  state?: string;
  desiredState?: string;
  containerId?: string;
  error?: string;
  timestamp?: string;
}

/** Swarm service state carried by "service" nodes. */
export interface ServiceInfo {
  /** "replicated" | "global" | "replicated-job" | "global-job". */
  mode?: string;
  replicas: ReplicaCount;
  tasks?: TaskInfo[];
  /** Rolling-update state (e.g. "updating", "completed"); empty when none ran. */
  updateStatus?: string;
}

/** Swarm cluster node state carried by "swarmnode" graph nodes. */
export interface SwarmNodeInfo {
  id: string;
  role: 'manager' | 'worker';
  /** True for the raft leader among the managers. */
  leader?: boolean;
  availability: 'active' | 'pause' | 'drain';
  /** Docker node state: "ready", "down", "unknown", "disconnected". */
  state: string;
  addr?: string;
  engineVersion?: string;
  /** CPU capacity in nano-CPUs (1e9 = one core). */
  nanoCpus?: number;
  memoryBytes?: number;
}

export interface DGNode {
  id: string;
  type: 'container' | 'service' | 'network' | 'volume' | 'swarmnode';
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
  createdAt?: string;
  compose?: ComposeConfig;
  /** Compose project or swarm stack namespace the node belongs to. */
  stack?: string;
  service?: ServiceInfo;
  swarmNode?: SwarmNodeInfo;
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

export interface ServiceNodeData {
  dgNode: DGNode;
  nodeWidth?: number;
  stats?: import('./types/stats').ContainerStatsData;
  onInfoClick?: (serviceId: string) => void;
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

/** Swarm node role in the per-node graph view ("manager" or "worker"). */
export type SwarmRole = 'manager' | 'worker';

/** A role container ("Managers" / "Workers") of the per-node graph view. */
export interface RoleGroupData {
  role: SwarmRole;
  /** Number of swarm node boxes in the group. */
  nodeCount: number;
  /** Number of tasks (and local containers) across those boxes. */
  taskCount: number;
}

/**
 * Standalone badge of the per-node graph view, centered in the gap between
 * the Managers and Workers groups. The control link fans out to one spoke
 * per worker (see utils/swarmLinks.ts), so this carries the aggregate
 * "protocol · port · ready/total" reading that used to live on a single edge.
 */
export interface ControlSummaryData {
  ready: number;
  total: number;
  healthy: boolean;
}

/** A swarm node box in the per-node graph view. */
export interface SwarmNodeGroupData {
  /** The swarmnode graph node, or a synthetic one for the Unassigned group. */
  dgNode: DGNode;
  /** True for the group of tasks not yet placed on a node. */
  unassigned?: boolean;
  /** Role of the node, for its accent colour (absent for Unassigned). */
  role?: SwarmRole;
  /** Number of tasks (and local containers) placed in the box. */
  taskCount: number;
  /** Aggregate stats for the node (`node:{hostname}`), when an agent reports them. */
  stats?: import('./types/stats').ContainerStatsData;
  onInfoClick?: (nodeId: string) => void;
}

/**
 * One service on one swarm node in the per-node graph view: a card holding
 * the service's tasks placed on that node (id `nodesvc:{hostname}:{service}`).
 */
export interface NodeServiceCardData {
  /** Synthetic node carrying the service name and stack (search, minimap). */
  dgNode: DGNode;
  /** Graph id of the service (`service:{name}`). */
  serviceId: string;
  serviceName: string;
  stack?: string;
  /** The service's running-desired tasks on this node, by slot. */
  tasks: TaskInfo[];
  /** True when the hosting node is down, drained or paused. */
  nodeInactive?: boolean;
  /** Per-task live stats keyed by task id, injected by the canvas. */
  taskStats?: Record<string, import('./types/stats').ContainerStatsData>;
  /** True while a card of the same service (on any node) is hovered. */
  peerHover?: boolean;
  /** Opens the detail panel for the given graph node (the service). */
  onInfoClick?: (nodeId: string) => void;
}

/** A container using a named volume, with its mount path. */
export interface VolumeMount {
  node: DGNode;
  mountPath: string;
}

/** Typed payload for ELK-routed edges. */
export interface ElkEdgeData {
  path?: string;
  edgeType?: string;
  active?: boolean;
  animated?: boolean;
  nodeCount?: number;
}

export type { ContainerStatsData, StatsMessage, ContainerDetail, Mount, LogLine, VolumeDetail, NetworkDetail } from './types/stats';
