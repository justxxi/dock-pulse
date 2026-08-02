import { describe, it, expect, vi } from 'vitest';
import { Transport } from './transport';

describe('Transport reconnection logic', () => {
  it('instantiates transport cleanly', () => {
    const t = new Transport('ws://localhost:8080/api/stream');
    expect(t).toBeDefined();
  });

  it('notifies status listeners on connect attempt', () => {
    const t = new Transport('ws://localhost:8080/api/stream');
    const statusFn = vi.fn();
    t.onStatusChange(statusFn);
    t.connect();
    expect(statusFn).toHaveBeenCalledWith('connecting', 0);
  });
});
