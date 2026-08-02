export interface Port {
  ip?: string;
  private_port: number;
  public_port?: number;
  type: string;
}

export interface ContainerState {
  status: string;
  running: boolean;
  paused: boolean;
  restarting: boolean;
  oom_killed: boolean;
  dead: boolean;
  pid: number;
  exit_code: number;
  error: string;
  started_at: string;
  finished_at: string;
}

export interface Container {
  id: string;
  name: string;
  image: string;
  image_id: string;
  command: string;
  created: string;
  state: ContainerState;
  status: string;
  ports: Port[];
  labels: Record<string, string>;
  mounts: string[];
  network: string;
  restart_count: number;
}

export interface StatsPoint {
  cpu_percent: number;
  memory_bytes: number;
  memory_limit: number;
  memory_percent: number;
  net_rx_bytes: number;
  net_tx_bytes: number;
  block_read: number;
  block_write: number;
  timestamp: number;
}

export interface LogLine {
  container_id: string;
  seq: number;
  timestamp: string;
  stream: 'stdout' | 'stderr';
  text: string;
}

export interface SupervisorEvent {
  container_id: string;
  action: 'scheduled' | 'restarting' | 'exhausted';
  attempt: number;
  next_retry?: string;
  reason: string;
  exhausted: boolean;
}

export interface SnapshotData {
  version: string;
  containers: Container[];
  seq: number;
}

export interface ContainerUpdatedData {
  container: Container;
}

export interface ContainerRemovedData {
  id: string;
}

export interface ContainerStatsData {
  id: string;
  stats: StatsPoint;
}

export interface Envelope<T = unknown> {
  type: string;
  seq?: number;
  data: T;
}
