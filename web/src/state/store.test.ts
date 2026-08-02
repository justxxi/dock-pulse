import { describe, it, expect, beforeEach } from 'vitest';
import { store } from './store';
import { Container } from '../api/protocol';

describe('Store state management', () => {
  beforeEach(() => {
    store.setSnapshot([]);
  });

  it('updates container snapshot and single container', () => {
    const c1: Container = {
      id: 'c1',
      name: 'web',
      image: 'nginx',
      image_id: 'img1',
      command: 'nginx',
      created: new Date().toISOString(),
      state: { status: 'running', running: true, paused: false, restarting: false, oom_killed: false, dead: false, pid: 1, exit_code: 0, error: '', started_at: '', finished_at: '' },
      status: 'Up 1h',
      ports: [],
      labels: {},
      mounts: [],
      network: 'bridge',
      restart_count: 0,
    };

    store.setSnapshot([c1]);
    expect(Object.keys(store.getState().containers).length).toBe(1);

    const updatedC1 = { ...c1, status: 'Up 2h' };
    store.updateContainer(updatedC1);
    expect(store.getState().containers['c1']?.status).toBe('Up 2h');
  });

  it('removes container', () => {
    const c1: Container = {
      id: 'c1',
      name: 'web',
      image: 'nginx',
      image_id: 'img1',
      command: 'nginx',
      created: new Date().toISOString(),
      state: { status: 'running', running: true, paused: false, restarting: false, oom_killed: false, dead: false, pid: 1, exit_code: 0, error: '', started_at: '', finished_at: '' },
      status: 'Up 1h',
      ports: [],
      labels: {},
      mounts: [],
      network: 'bridge',
      restart_count: 0,
    };

    store.setSnapshot([c1]);
    store.removeContainer('c1');
    expect(store.getState().containers['c1']).toBeUndefined();
  });
});
