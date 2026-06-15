export interface ContainerStatsData {
  cpuPercent: number;
  cpuThrottled: number;
  memUsage: number;
  memLimit: number;
  netRx: number;
  netTx: number;
  netRxErrors: number;
  netTxErrors: number;
  blockRead: number;
  blockWrite: number;
  pids: number;
}

export interface StatsMessage {
  stats: Record<string, ContainerStatsData>;
}

/**
 * A volume or bind mount. Shared by running containers (from inspect) and
 * not-yet-started compose services (from the compose file). For named volumes,
 * `name` is the fully-qualified volume node name used to link to the graph node;
 * for bind mounts it is absent and `source` holds the host path.
 */
export interface Mount {
  type: string;
  source: string;
  destination: string;
  rw: boolean;
  name?: string;
  propagation?: string;
}

export interface ContainerDetail {
  name: string;
  image: string;
  status: string;
  running: boolean;
  paused: boolean;
  restarting: boolean;
  oomKilled: boolean;
  pid: number;
  exitCode: number;
  startedAt: string;
  finishedAt: string;
  cmd: string[];
  entrypoint: string[];
  workingDir: string;
  user: string;
  env: { key: string; value: string }[];
  labels: Record<string, string>;
  restartPolicy: { Name: string; MaximumRetryCount: number };
  networkMode: string;
  ports: { hostPort: string; containerPort: string; protocol: string }[];
  mounts: Mount[];
  networks: { name: string; ipAddress: string; gateway: string; macAddress: string; ipPrefixLen: number }[];
  security: { privileged: boolean; readonlyRootfs: boolean; capAdd: string[]; capDrop: string[] };
  resources: { cpuQuota: number; cpuPeriod: number; nanoCpus: number; memoryLimit: number; memoryReservation: number };
  health?: { status: string; failingStreak: number; log: { start: string; end: string; exitCode: number; output: string }[] };
}

export interface LogLine {
  id: number;
  stream: 'stdout' | 'stderr';
  text: string;
  timestamp?: string;
  /** Source container name — set only for the aggregate (multi-container) stream. */
  container?: string;
}

export interface VolumeDetail {
  name: string;
  driver: string;
  mountpoint: string;
  scope: string;
  createdAt: string;
  labels: Record<string, string>;
  options: Record<string, string>;
  usageSize: number;
  usageRefCount: number;
  status: Record<string, string>;
}

export interface NetworkDetail {
  name: string;
  id: string;
  driver: string;
  scope: string;
  internal: boolean;
  enableIPv6: boolean;
  created: string;
  ipam: {
    driver: string;
    config: { subnet: string; gateway: string; ipRange: string; auxAddresses: Record<string, string> }[];
  };
  options: Record<string, string>;
  labels: Record<string, string>;
  containers: { name: string; ipv4Address: string; ipv6Address: string; macAddress: string }[];
}
