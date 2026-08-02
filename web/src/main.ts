import { h } from './lib/dom';
import { store, AppState } from './state/store';
import { transport } from './api/transport';
import { Envelope, SnapshotData, ContainerUpdatedData, ContainerRemovedData, ContainerStatsData, LogLine, SupervisorEvent } from './api/protocol';
import { createContainersView } from './views/containers';
import { createDetailView } from './views/detail';
import { createPaletteView } from './views/palette';
import { showToast } from './components/toast';

function initApp(): void {
  const root = document.getElementById('app');
  if (!root) return;

  const brandDot = h('span', { className: 'brand-dot' });
  const brandTitle = h('span', null, 'dock-pulse');
  const brand = h('div', { className: 'brand' }, brandDot, brandTitle);

  const themeBtn = h('button', {
    className: 'btn',
    onClick: () => {
      const current = store.getState().theme;
      store.setTheme(current === 'dark' ? 'light' : 'dark');
    },
  }, 'Theme');

  const paletteBtn = h('button', {
    className: 'btn btn-primary',
    onClick: () => store.setPaletteOpen(true),
  }, 'Cmd+K');

  const headerActions = h('div', { className: 'header-actions' }, themeBtn, paletteBtn);
  const header = h('header', { className: 'app-header' }, brand, headerActions);

  const offlineBanner = h('div', { className: 'offline-banner', style: { display: 'none' } },
    h('span', null, 'Connection lost. Reconnecting...'),
    h('button', {
      className: 'btn',
      onClick: () => transport.connect(),
    }, 'Retry Now')
  );

  const containersView = createContainersView();
  const detailView = createDetailView();
  const paletteView = createPaletteView();

  const content = h('main', { className: 'app-content' }, containersView, detailView);

  root.appendChild(header);
  root.appendChild(offlineBanner);
  root.appendChild(content);
  root.appendChild(paletteView);

  store.subscribe((state: AppState) => {
    if (state.connectionStatus === 'disconnected') {
      offlineBanner.style.display = 'flex';
      brandDot.style.backgroundColor = 'var(--color-error)';
    } else if (state.connectionStatus === 'connecting') {
      offlineBanner.style.display = 'flex';
      brandDot.style.backgroundColor = 'var(--color-warning)';
    } else {
      offlineBanner.style.display = 'none';
      brandDot.style.backgroundColor = 'var(--color-success)';
    }
  });

  transport.onStatusChange((status, count) => {
    store.setConnectionStatus(status, count);
  });

  transport.subscribe((env: Envelope) => {
    switch (env.type) {
      case 'snapshot': {
        const data = env.data as SnapshotData;
        store.setSnapshot(data.containers || []);
        break;
      }
      case 'container.updated': {
        const data = env.data as ContainerUpdatedData;
        store.updateContainer(data.container);
        break;
      }
      case 'container.removed': {
        const data = env.data as ContainerRemovedData;
        store.removeContainer(data.id);
        break;
      }
      case 'stats': {
        const data = env.data as ContainerStatsData;
        store.addStats(data.id, data.stats);
        break;
      }
      case 'log': {
        const data = env.data as LogLine;
        store.addLogLines(data.container_id, [data]);
        break;
      }
      case 'supervisor': {
        const data = env.data as SupervisorEvent;
        if (data.action === 'exhausted') {
          showToast(`Supervisor exhausted restarts for ${data.container_id}`, 'error', 5000);
        } else if (data.action === 'restarting') {
          showToast(`Supervisor restarting container ${data.container_id}`, 'info', 3000);
        }
        break;
      }
    }
  });

  transport.connect();
}

document.addEventListener('DOMContentLoaded', initApp);
