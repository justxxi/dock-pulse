import { AppState } from './store';
import { Container, StatsPoint } from '../api/protocol';

export function selectFilteredContainers(state: AppState): Container[] {
  const query = state.searchQuery.toLowerCase().trim();
  const filter = state.statusFilter;

  const list = Object.values(state.containers);

  return list.filter(c => {
    if (filter === 'running' && !c.state.running) return false;
    if (filter === 'exited' && c.state.running) return false;

    if (query !== '') {
      const matchName = c.name.toLowerCase().includes(query);
      const matchImage = c.image.toLowerCase().includes(query);
      const matchId = c.id.toLowerCase().includes(query);
      if (!matchName && !matchImage && !matchId) return false;
    }

    return true;
  }).sort((a, b) => a.name.localeCompare(b.name));
}

export function selectContainerById(state: AppState, id: string | null): Container | null {
  if (!id) return null;
  return state.containers[id] || null;
}

export function selectLatestStats(state: AppState, id: string): StatsPoint | null {
  const points = state.stats[id];
  if (!points || points.length === 0) return null;
  return points[points.length - 1] || null;
}
