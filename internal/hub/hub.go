package hub

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/coder/websocket"
	"github.com/justxxi/dock-pulse/internal/protocol"
)

var ErrMaxConnections = errors.New("maximum websocket connections reached")

type Client struct {
	ID             string
	conn           *websocket.Conn
	Send           chan []byte
	subscribedLogs map[string]struct{}
	hub            *Hub
	cancel         context.CancelFunc
}

func NewClient(id string, conn *websocket.Conn, hub *Hub, bufSize int, cancel context.CancelFunc) *Client {
	if bufSize <= 0 {
		bufSize = 256
	}
	return &Client{
		ID:             id,
		conn:           conn,
		Send:           make(chan []byte, bufSize),
		subscribedLogs: make(map[string]struct{}),
		hub:            hub,
		cancel:         cancel,
	}
}

type Hub struct {
	mu             sync.RWMutex
	logger         *slog.Logger
	clients        map[string]*Client
	maxConnections int
	count          atomic.Int64
}

func NewHub(logger *slog.Logger, maxConnections int) *Hub {
	if maxConnections <= 0 {
		maxConnections = 100
	}
	return &Hub{
		logger:         logger,
		clients:        make(map[string]*Client),
		maxConnections: maxConnections,
	}
}

func (h *Hub) Register(c *Client) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.clients) >= h.maxConnections {
		return ErrMaxConnections
	}

	h.clients[c.ID] = c
	h.count.Add(1)
	h.logger.Debug("registered websocket client", "client_id", c.ID, "total_clients", len(h.clients))
	return nil
}

func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[c.ID]; ok {
		delete(h.clients, c.ID)
		h.count.Add(-1)
		close(c.Send)
		if c.cancel != nil {
			c.cancel()
		}
		h.logger.Debug("unregistered websocket client", "client_id", c.ID, "total_clients", len(h.clients))
	}
}

func (h *Hub) BroadcastState(msgType string, seq uint64, payload interface{}) {
	data, err := protocol.EncodeEnvelope(msgType, seq, payload)
	if err != nil {
		h.logger.Error("failed to encode broadcast payload", "type", msgType, "error", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, c := range h.clients {
		h.sendNonBlocking(c, data, msgType)
	}
}

func (h *Hub) BroadcastLog(containerID string, line protocol.LogLineData) {
	data, err := protocol.EncodeEnvelope(protocol.TypeLog, 0, line)
	if err != nil {
		h.logger.Error("failed to encode log payload", "container_id", containerID, "error", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, c := range h.clients {
		if _, subscribed := c.subscribedLogs[containerID]; subscribed {
			h.sendNonBlocking(c, data, protocol.TypeLog)
		}
	}
}

func (h *Hub) sendNonBlocking(c *Client, data []byte, msgType string) {
	select {
	case c.Send <- data:
	default:
		if msgType == protocol.TypeStats {
			return
		}
		h.logger.Warn("slow client buffer overflow, dropping client connection", "client_id", c.ID)
		c.conn.Close(websocket.StatusPolicyViolation, "slow client buffer overflow")
	}
}

func (h *Hub) SubscribeLogs(client *Client, containerID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	client.subscribedLogs[containerID] = struct{}{}
}

func (h *Hub) UnsubscribeLogs(client *Client, containerID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(client.subscribedLogs, containerID)
}

func (h *Hub) ClientCount() int {
	return int(h.count.Load())
}
