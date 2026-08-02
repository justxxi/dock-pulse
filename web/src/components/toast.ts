import { h } from '../lib/dom';

let container: HTMLElement | null = null;

function getContainer(): HTMLElement {
  if (!container) {
    container = h('div', {
      className: 'toast-container',
      'aria-live': 'polite',
      'aria-atomic': 'true',
    });
    document.body.appendChild(container);
  }
  return container;
}

export function showToast(message: string, type: 'info' | 'error' | 'success' = 'info', duration = 3000): void {
  const c = getContainer();

  const toast = h('div', {
    className: `toast ${type === 'error' ? 'toast-error' : ''}`,
  }, message);

  c.appendChild(toast);

  setTimeout(() => {
    toast.remove();
  }, duration);
}
