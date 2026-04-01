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
  mounts: { type: string; source: string; destination: string; rw: boolean; propagation: string; name?: string }[];
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
