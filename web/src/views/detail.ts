import { h, clearChildren } from '../lib/dom';
import { formatUptime, truncateId } from '../lib/format';
import { selectContainerById } from '../state/selectors';
import { store, AppState } from '../state/store';
import { createLogsView } from './logs';

export function createDetailView(): HTMLElement {
  let activeTab: 'overview' | 'logs' = 'overview';

  const container = h('div', { className: 'detail-pane' });

  function render(state: AppState): void {
    clearChildren(container);

    const c = selectContainerById(state, state.selectedContainerId);
    if (!c) {
      container.style.display = 'none';
      return;
    }

    container.style.display = 'flex';

    const isRunning = c.state.running;
    const statusClass = isRunning ? 'status-running' : 'status-exited';
    const statusText = isRunning ? 'Running' : 'Exited';

    const header = h('div', { className: 'detail-header' },
      h('div', null,
        h('h3', { style: { fontSize: '16px', fontWeight: '600' } }, c.name),
        h('span', { className: `status-badge ${statusClass}`, style: { marginTop: '4px' } }, statusText)
      ),
      h('button', {
        className: 'btn',
        onClick: () => store.setSelectedContainer(null),
      }, '✕')
    );

    const tabOverview = h('button', {
      className: `tab-btn ${activeTab === 'overview' ? 'active' : ''}`,
      onClick: () => {
        activeTab = 'overview';
        render(store.getState());
      },
    }, 'Overview');

    const tabLogs = h('button', {
      className: `tab-btn ${activeTab === 'logs' ? 'active' : ''}`,
      onClick: () => {
        activeTab = 'logs';
        render(store.getState());
      },
    }, 'Logs');

    const tabsNav = h('div', { className: 'detail-tabs' }, tabOverview, tabLogs);

    const contentArea = h('div', { style: { flex: '1', overflowY: 'auto', display: 'flex', flexDirection: 'column' } });

    if (activeTab === 'overview') {
      const portsText = c.ports.map(p => `${p.public_port ? `${p.public_port}:` : ''}${p.private_port}/${p.type}`).join(', ') || 'None';
      const mountsText = c.mounts.join('\n') || 'None';

      const overviewEl = h('div', { style: { padding: '16px', display: 'flex', flexDirection: 'column', gap: '12px' } },
        h('div', null,
          h('div', { style: { fontSize: '12px', color: 'var(--color-text-secondary)' } }, 'Container ID'),
          h('div', { className: 'font-mono' }, truncateId(c.id))
        ),
        h('div', null,
          h('div', { style: { fontSize: '12px', color: 'var(--color-text-secondary)' } }, 'Image'),
          h('div', { className: 'font-mono' }, c.image)
        ),
        h('div', null,
          h('div', { style: { fontSize: '12px', color: 'var(--color-text-secondary)' } }, 'Command'),
          h('div', { className: 'font-mono', style: { wordBreak: 'break-all' } }, c.command || '-')
        ),
        h('div', null,
          h('div', { style: { fontSize: '12px', color: 'var(--color-text-secondary)' } }, 'Uptime'),
          h('div', null, formatUptime(c.created))
        ),
        h('div', null,
          h('div', { style: { fontSize: '12px', color: 'var(--color-text-secondary)' } }, 'Ports'),
          h('div', { className: 'font-mono' }, portsText)
        ),
        h('div', null,
          h('div', { style: { fontSize: '12px', color: 'var(--color-text-secondary)' } }, 'Mounts'),
          h('pre', { className: 'font-mono', style: { whiteSpace: 'pre-wrap', fontSize: '12px' } }, mountsText)
        ),
        h('div', null,
          h('div', { style: { fontSize: '12px', color: 'var(--color-text-secondary)' } }, 'Restart Count'),
          h('div', { className: 'num-tabular' }, String(c.restart_count))
        )
      );
      contentArea.appendChild(overviewEl);
    } else {
      contentArea.appendChild(createLogsView(c.id));
    }

    container.appendChild(header);
    container.appendChild(tabsNav);
    container.appendChild(contentArea);
  }

  store.subscribe(render);
  render(store.getState());

  return container;
}
