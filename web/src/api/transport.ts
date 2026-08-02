import { Envelope } from './protocol';

export type TransportListener = (env: Envelope) => void;
export type StatusListener = (status: 'connected' | 'connecting' | 'disconnected', attempt: number) => void;

export class Transport {
  private ws: WebSocket | null = null;
  private url: string;
  private listeners: Set<TransportListener> = new Set();
  private statusListeners: Set<StatusListener> = new Set();
  private queue: string[] = [];
  private reconnectAttempt = 0;
  private maxReconnectDelay = 30000;
  private reconnectTimer: number | null = null;
  private isExplicitClose = false;

  constructor(url?: string) {
    if (url) {
      this.url = url;
    } else if (typeof window !== 'undefined') {
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      this.url = `${protocol}//${window.location.host}/api/stream`;
    } else {
      this.url = 'ws://127.0.0.1:8080/api/stream';
    }

    if (typeof document !== 'undefined') {
      document.addEventListener('visibilitychange', () => {
        if (!document.hidden && (!this.ws || this.ws.readyState === WebSocket.CLOSED)) {
          this.connect();
        }
      });
    }
  }

  connect(): void {
    if (typeof WebSocket === 'undefined') return;
    if (this.ws && (this.ws.readyState === WebSocket.OPEN || this.ws.readyState === WebSocket.CONNECTING)) {
      return;
    }

    this.isExplicitClose = false;
    this.notifyStatus('connecting', this.reconnectAttempt);

    try {
      this.ws = new WebSocket(this.url);

      this.ws.onopen = () => {
        this.reconnectAttempt = 0;
        this.notifyStatus('connected', 0);
        this.flushQueue();
      };

      this.ws.onmessage = (event: MessageEvent) => {
        try {
          const env = JSON.parse(event.data) as Envelope;
          this.notifyMessage(env);
        } catch {
          // ignore malformed message
        }
      };

      this.ws.onclose = () => {
        this.notifyStatus('disconnected', this.reconnectAttempt);
        if (!this.isExplicitClose) {
          this.scheduleReconnect();
        }
      };

      this.ws.onerror = () => {
        if (this.ws) {
          this.ws.close();
        }
      };
    } catch {
      this.notifyStatus('disconnected', this.reconnectAttempt);
      this.scheduleReconnect();
    }
  }

  disconnect(): void {
    this.isExplicitClose = true;
    if (this.reconnectTimer !== null && typeof window !== 'undefined') {
      window.clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }

  send(type: string, data: unknown): void {
    const payload = JSON.stringify({ type, data });
    if (this.ws && typeof WebSocket !== 'undefined' && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(payload);
    } else {
      this.queue.push(payload);
    }
  }

  subscribe(listener: TransportListener): () => void {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }

  onStatusChange(listener: StatusListener): () => void {
    this.statusListeners.add(listener);
    return () => this.statusListeners.delete(listener);
  }

  private flushQueue(): void {
    while (this.queue.length > 0 && this.ws && typeof WebSocket !== 'undefined' && this.ws.readyState === WebSocket.OPEN) {
      const msg = this.queue.shift();
      if (msg) this.ws.send(msg);
    }
  }

  private scheduleReconnect(): void {
    if (this.reconnectTimer !== null) return;
    this.reconnectAttempt++;
    const delay = Math.min(1000 * Math.pow(2, this.reconnectAttempt - 1), this.maxReconnectDelay);
    const jitter = Math.random() * 500;
    if (typeof window !== 'undefined') {
      this.reconnectTimer = window.setTimeout(() => {
        this.reconnectTimer = null;
        this.connect();
      }, delay + jitter);
    }
  }

  private notifyMessage(env: Envelope): void {
    for (const listener of this.listeners) {
      listener(env);
    }
  }

  private notifyStatus(status: 'connected' | 'connecting' | 'disconnected', attempt: number): void {
    for (const listener of this.statusListeners) {
      listener(status, attempt);
    }
  }
}

export const transport = new Transport();
