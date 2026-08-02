import { describe, it, expect, vi, beforeEach } from 'vitest';
import { Transport } from './transport';

class MockWebSocket {
  static OPEN = 1;
  static CONNECTING = 0;
  static CLOSED = 3;
  readyState = MockWebSocket.CONNECTING;
  onopen: (() => void) | null = null;
  onmessage: ((e: unknown) => void) | null = null;
  onclose: (() => void) | null = null;
  onerror: (() => void) | null = null;
  constructor(public url: string) {}
  close() {}
  send() {}
}

beforeEach(() => {
  (globalThis as unknown as { WebSocket: typeof MockWebSocket }).WebSocket = MockWebSocket;
});

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
