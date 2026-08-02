import { h, clearChildren } from '../lib/dom';
import { store, AppState } from '../state/store';

export function createPaletteView(): HTMLElement {
  let selectedIndex = 0;
  let query = '';

  const input = h('input', {
    type: 'text',
    className: 'palette-input',
    placeholder: 'Type a command or search container...',
  }) as HTMLInputElement;

  const resultsList = h('div', { className: 'palette-results' });

  const dialog = h('div', { className: 'palette-dialog' }, input, resultsList);
  const overlay = h('div', { className: 'palette-overlay' }, dialog);

  overlay.style.display = 'none';

  document.addEventListener('keydown', (e: KeyboardEvent) => {
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
      e.preventDefault();
      const current = store.getState().paletteOpen;
      store.setPaletteOpen(!current);
    }
  });

  input.addEventListener('input', (e: Event) => {
    query = (e.target as HTMLInputElement).value.toLowerCase().trim();
    selectedIndex = 0;
    render(store.getState());
  });

  input.addEventListener('keydown', (e: KeyboardEvent) => {
    if (e.key === 'Escape') {
      store.setPaletteOpen(false);
      return;
    }

    const items = resultsList.children;
    if (items.length === 0) return;

    if (e.key === 'ArrowDown') {
      e.preventDefault();
      selectedIndex = (selectedIndex + 1) % items.length;
      updateSelection();
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      selectedIndex = (selectedIndex - 1 + items.length) % items.length;
      updateSelection();
    } else if (e.key === 'Enter') {
      e.preventDefault();
      const selected = items[selectedIndex] as HTMLElement | undefined;
      if (selected) {
        selected.click();
      }
    }
  });

  function updateSelection(): void {
    const items = resultsList.children;
    for (let i = 0; i < items.length; i++) {
      const item = items[i] as HTMLElement;
      if (i === selectedIndex) {
        item.classList.add('selected');
        item.scrollIntoView({ block: 'nearest' });
      } else {
        item.classList.remove('selected');
      }
    }
  }

  function render(state: AppState): void {
    if (!state.paletteOpen) {
      overlay.style.display = 'none';
      return;
    }

    overlay.style.display = 'flex';
    input.focus();

    clearChildren(resultsList);

    const actions: Array<{ title: string; subtitle?: string; action: () => void }> = [];

    const currentTheme = state.theme;
    const nextTheme = currentTheme === 'dark' ? 'light' : 'dark';
    actions.push({
      title: `Switch Theme to ${nextTheme}`,
      subtitle: `Current theme: ${currentTheme}`,
      action: () => {
        store.setTheme(nextTheme);
        store.setPaletteOpen(false);
      },
    });

    const containers = Object.values(state.containers);
    for (const c of containers) {
      if (query === '' || c.name.toLowerCase().includes(query) || c.image.toLowerCase().includes(query)) {
        actions.push({
          title: `Focus: ${c.name}`,
          subtitle: `Image: ${c.image}`,
          action: () => {
            store.setSelectedContainer(c.id);
            store.setPaletteOpen(false);
          },
        });
      }
    }

    if (actions.length === 0) {
      resultsList.appendChild(
        h('div', { style: { padding: '16px', color: 'var(--color-text-muted)', textAlign: 'center' } }, 'No matching commands')
      );
      return;
    }

    actions.forEach((act, idx) => {
      const item = h('div', {
        className: `palette-item ${idx === selectedIndex ? 'selected' : ''}`,
        onClick: () => act.action(),
      },
        h('div', null,
          h('div', { style: { fontWeight: '500' } }, act.title),
          act.subtitle ? h('div', { style: { fontSize: '12px', color: 'var(--color-text-muted)' } }, act.subtitle) : null
        )
      );
      resultsList.appendChild(item);
    });
  }

  store.subscribe(render);

  return overlay;
}
