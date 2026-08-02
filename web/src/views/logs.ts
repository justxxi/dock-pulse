import { h, clearChildren } from '../lib/dom';
import { ansiToHtml } from '../lib/ansi';
import { formatTimestamp } from '../lib/format';
import { store, AppState } from '../state/store';

export function createLogsView(containerId: string): HTMLElement {
  let isFollow = true;
  let filterText = '';
  let streamFilter: 'all' | 'stdout' | 'stderr' = 'all';

  const logContainer = h('div', { className: 'log-viewer' });

  logContainer.addEventListener('scroll', () => {
    const isAtBottom = logContainer.scrollHeight - logContainer.clientHeight - logContainer.scrollTop < 20;
    isFollow = isAtBottom;
  });

  function renderLogs(state: AppState): void {
    const lines = state.logs[containerId] || [];
    clearChildren(logContainer);

    const filtered = lines.filter(l => {
      if (streamFilter !== 'all' && l.stream !== streamFilter) return false;
      if (filterText !== '' && !l.text.toLowerCase().includes(filterText.toLowerCase())) return false;
      return true;
    });

    if (filtered.length === 0) {
      logContainer.appendChild(
        h('div', { style: { color: 'var(--color-text-muted)', padding: '16px' } }, 'No logs available')
      );
      return;
    }

    const fragment = document.createDocumentFragment();
    for (const l of filtered) {
      const lineEl = h('div', { className: `log-line log-stream-${l.stream}` });

      const tsEl = h('span', { className: 'log-timestamp' }, formatTimestamp(l.timestamp));
      lineEl.appendChild(tsEl);

      const htmlText = ansiToHtml(l.text);
      const textSpan = h('span');
      textSpan.innerHTML = htmlText;
      lineEl.appendChild(textSpan);

      fragment.appendChild(lineEl);
    }

    logContainer.appendChild(fragment);

    if (isFollow) {
      logContainer.scrollTop = logContainer.scrollHeight;
    }
  }

  store.subscribe(renderLogs);
  renderLogs(store.getState());

  return logContainer;
}
