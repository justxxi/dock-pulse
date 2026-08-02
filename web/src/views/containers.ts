import { h, clearChildren } from '../lib/dom';
import { formatBytes, formatUptime } from '../lib/format';
import { selectFilteredContainers, selectLatestStats } from '../state/selectors';
import { store, AppState } from '../state/store';
import { renderSparkline } from '../components/sparkline';
import { showToast } from '../components/toast';

let animFrameRequested = false;

export function createContainersView(): HTMLElement {
  const searchInput = h('input', {
    type: 'text',
    className: 'search-input',
    placeholder: 'Filter containers (Cmd/Ctrl+K)...',
    value: store.getState().searchQuery,
    onInput: (e: Event) => {
      store.setSearchQuery((e.target as HTMLInputElement).value);
    },
  });

  const filterSelect = h('select', {
    className: 'filter-select',
    onChange: (e: Event) => {
      store.setStatusFilter((e.target as HTMLSelectElement).value as 'all' | 'running' | 'exited');
    },
  },
    h('option', { value: 'all' }, 'All Statuses'),
    h('option', { value: 'running' }, 'Running'),
    h('option', { value: 'exited' }, 'Exited')
  );

  const toolbar = h('div', { className: 'toolbar' }, searchInput, filterSelect);
  const tableBody = h('tbody');

  const table = h('table', { className: 'container-table' },
    h('thead', null,
      h('tr', null,
        h('th', null, 'Status'),
        h('th', null, 'Name'),
        h('th', null, 'Image'),
        h('th', null, 'Uptime'),
        h('th', null, 'CPU'),
        h('th', null, 'Memory'),
        h('th', null, 'Actions')
      )
    ),
    tableBody
  );

  const container = h('div', { className: 'containers-view' }, toolbar, table);

  function update(state: AppState): void {
    if (animFrameRequested) return;
    animFrameRequested = true;

    requestAnimationFrame(() => {
      animFrameRequested = false;
      clearChildren(tableBody);

      const list = selectFilteredContainers(state);

      if (list.length === 0) {
        tableBody.appendChild(
          h('tr', null,
            h('td', { colSpan: 7, style: { textAlign: 'center', color: 'var(--color-text-muted)', padding: '24px' } },
              'No containers found'
            )
          )
        );
        return;
      }

      for (const c of list) {
        const isRunning = c.state.running;
        const statusClass = isRunning ? 'status-running' : 'status-exited';
        const statusText = isRunning ? '● Running' : '■ Exited';

        const statsHistory = state.stats[c.id] || [];
        const cpuHistory = statsHistory.map(s => s.cpu_percent);
        const memHistory = statsHistory.map(s => s.memory_bytes);

        const latestStats = selectLatestStats(state, c.id);
        const cpuText = latestStats ? `${latestStats.cpu_percent.toFixed(1)}%` : '-';
        const memText = latestStats ? formatBytes(latestStats.memory_bytes) : '-';

        const cpuSparkline = renderSparkline(cpuHistory, '#6366f1');
        const memSparkline = renderSparkline(memHistory, '#10b981');

        const row = h('tr', {
          className: 'container-row',
          onClick: () => {
            store.setSelectedContainer(c.id);
          },
        },
          h('td', null, h('span', { className: `status-badge ${statusClass}` }, statusText)),
          h('td', { style: { fontWeight: '600' } }, c.name),
          h('td', { className: 'font-mono', style: { fontSize: '12px', color: 'var(--color-text-secondary)' } }, c.image),
          h('td', { className: 'num-tabular' }, formatUptime(c.created)),
          h('td', null,
            h('div', { style: { display: 'flex', alignItems: 'center', gap: '8px' } },
              h('span', { className: 'num-tabular font-mono', style: { width: '45px' } }, cpuText),
              cpuSparkline
            )
          ),
          h('td', null,
            h('div', { style: { display: 'flex', alignItems: 'center', gap: '8px' } },
              h('span', { className: 'num-tabular font-mono', style: { width: '60px' } }, memText),
              memSparkline
            )
          ),
          h('td', null,
            h('div', { style: { display: 'flex', gap: '6px' } },
              isRunning
                ? h('button', {
                  className: 'btn btn-danger',
                  onClick: (e: Event) => {
                    e.stopPropagation();
                    if (confirm(`Stop container ${c.name}?`)) {
                      fetch(`/api/containers/${c.id}/stop`, { method: 'POST' })
                        .then(res => {
                          if (!res.ok) throw new Error('Stop failed');
                          showToast(`Stopped ${c.name}`, 'info');
                        })
                        .catch(() => showToast(`Failed to stop ${c.name}`, 'error'));
                    }
                  },
                }, 'Stop')
                : h('button', {
                  className: 'btn btn-primary',
                  onClick: (e: Event) => {
                    e.stopPropagation();
                    fetch(`/api/containers/${c.id}/start`, { method: 'POST' })
                      .then(res => {
                        if (!res.ok) throw new Error('Start failed');
                        showToast(`Started ${c.name}`, 'info');
                      })
                      .catch(() => showToast(`Failed to start ${c.name}`, 'error'));
                  },
                }, 'Start')
            )
          )
        );

        tableBody.appendChild(row);
      }
    });
  }

  store.subscribe(update);
  update(store.getState());

  return container;
}
