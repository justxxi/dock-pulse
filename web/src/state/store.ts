import { Container, StatsPoint, LogLine } from '../api/protocol';

export interface AppState {
  containers: Record<string, Container>;
  stats: Record<string, StatsPoint[]>;
  logs: Record<string, LogLine[]>;
  selectedContainerId: string | null;
  statusFilter: 'all' | 'running' | 'exited';
  searchQuery: string;
  connectionStatus: 'connected' | 'connecting' | 'disconnected';
  reconnectCount: number;
  theme: 'dark' | 'light';
  paletteOpen: boolean;
}

export type Listener = (state: AppState) => void;

class Store {
  private state: AppState;
  private listeners: Set<Listener> = new Set();

  constructor() {
    const savedTheme = (typeof localStorage !== 'undefined' ? localStorage.getItem('theme') as 'dark' | 'light' : null) || 'dark';
    this.state = {
      containers: {},
      stats: {},
      logs: {},
      selectedContainerId: null,
      statusFilter: 'all',
      searchQuery: '',
      connectionStatus: 'disconnected',
      reconnectCount: 0,
      theme: savedTheme,
      paletteOpen: false,
    };
    if (typeof document !== 'undefined') {
      document.documentElement.setAttribute('data-theme', savedTheme);
    }
  }

  getState(): AppState {
    return this.state;
  }

  subscribe(listener: Listener): () => void {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }

  private setState(partial: Partial<AppState>): void {
    this.state = { ...this.state, ...partial };
    this.notify();
  }

  private notify(): void {
    for (const listener of this.listeners) {
      listener(this.state);
    }
  }

  setSnapshot(containers: Container[]): void {
    const map: Record<string, Container> = {};
    for (const c of containers) {
      map[c.id] = c;
    }
    this.setState({ containers: map });
  }

  updateContainer(container: Container): void {
    const containers = { ...this.state.containers, [container.id]: container };
    this.setState({ containers });
  }

  removeContainer(id: string): void {
    const containers = { ...this.state.containers };
    delete containers[id];
    const stats = { ...this.state.stats };
    delete stats[id];
    const logs = { ...this.state.logs };
    delete logs[id];

    let selectedContainerId = this.state.selectedContainerId;
    if (selectedContainerId === id) {
      selectedContainerId = null;
    }

    this.setState({ containers, stats, logs, selectedContainerId });
  }

  addStats(id: string, point: StatsPoint): void {
    const current = this.state.stats[id] || [];
    const updated = [...current, point].slice(-60);
    this.setState({
      stats: { ...this.state.stats, [id]: updated },
    });
  }

  addLogLines(containerId: string, lines: LogLine[]): void {
    const current = this.state.logs[containerId] || [];
    const updated = [...current, ...lines].slice(-2000);
    this.setState({
      logs: { ...this.state.logs, [containerId]: updated },
    });
  }

  setSelectedContainer(id: string | null): void {
    this.setState({ selectedContainerId: id });
  }

  setStatusFilter(filter: 'all' | 'running' | 'exited'): void {
    this.setState({ statusFilter: filter });
  }

  setSearchQuery(query: string): void {
    this.setState({ searchQuery: query });
  }

  setConnectionStatus(status: 'connected' | 'connecting' | 'disconnected', count?: number): void {
    this.setState({
      connectionStatus: status,
      reconnectCount: count !== undefined ? count : this.state.reconnectCount,
    });
  }

  setTheme(theme: 'dark' | 'light'): void {
    if (typeof localStorage !== 'undefined') {
      localStorage.setItem('theme', theme);
    }
    if (typeof document !== 'undefined') {
      document.documentElement.setAttribute('data-theme', theme);
    }
    this.setState({ theme });
  }

  setPaletteOpen(open: boolean): void {
    this.setState({ paletteOpen: open });
  }
}

export const store = new Store();
