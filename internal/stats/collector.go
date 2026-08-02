package stats

import (
	"bufio"
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/justxxi/dock-pulse/internal/dockerx"
	"github.com/justxxi/dock-pulse/internal/protocol"
)

type HistoryBuffer struct {
	points []protocol.StatsPoint
	head   int
	count  int
	size   int
}

func NewHistoryBuffer(size int) *HistoryBuffer {
	return &HistoryBuffer{
		points: make([]protocol.StatsPoint, size),
		size:   size,
	}
}

func (h *HistoryBuffer) Add(p protocol.StatsPoint) {
	h.points[h.head] = p
	h.head = (h.head + 1) % h.size
	if h.count < h.size {
		h.count++
	}
}

func (h *HistoryBuffer) Slice() []protocol.StatsPoint {
	res := make([]protocol.StatsPoint, h.count)
	if h.count < h.size {
		copy(res, h.points[:h.count])
		return res
	}
	start := h.head
	for i := 0; i < h.size; i++ {
		res[i] = h.points[(start+i)%h.size]
	}
	return res
}

type Collector struct {
	mu          sync.RWMutex
	engine      dockerx.Engine
	logger      *slog.Logger
	interval    time.Duration
	historySize int
	history     map[string]*HistoryBuffer
	cancelFuncs map[string]context.CancelFunc
	sem         chan struct{}
	onStats     func(id string, pt protocol.StatsPoint)
}

func NewCollector(engine dockerx.Engine, logger *slog.Logger, interval time.Duration, historySize int, maxConcurrency int, onStats func(id string, pt protocol.StatsPoint)) *Collector {
	if maxConcurrency <= 0 {
		maxConcurrency = 20
	}
	if historySize <= 0 {
		historySize = 60
	}
	return &Collector{
		engine:      engine,
		logger:      logger,
		interval:    interval,
		historySize: historySize,
		history:     make(map[string]*HistoryBuffer),
		cancelFuncs: make(map[string]context.CancelFunc),
		sem:         make(chan struct{}, maxConcurrency),
		onStats:     onStats,
	}
}

func (c *Collector) StartCollecting(ctx context.Context, id string) {
	c.mu.Lock()
	if _, exists := c.cancelFuncs[id]; exists {
		c.mu.Unlock()
		return
	}
	cCtx, cancel := context.WithCancel(ctx)
	c.cancelFuncs[id] = cancel
	if _, exists := c.history[id]; !exists {
		c.history[id] = NewHistoryBuffer(c.historySize)
	}
	c.mu.Unlock()

	go c.runContainerStream(cCtx, id)
}

func (c *Collector) StopCollecting(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if cancel, exists := c.cancelFuncs[id]; exists {
		cancel()
		delete(c.cancelFuncs, id)
	}
}

func (c *Collector) GetHistory(id string) []protocol.StatsPoint {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if buf, ok := c.history[id]; ok {
		return buf.Slice()
	}
	return nil
}

func (c *Collector) runContainerStream(ctx context.Context, id string) {
	select {
	case c.sem <- struct{}{}:
		defer func() { <-c.sem }()
	case <-ctx.Done():
		return
	}

	reader, err := c.engine.Stats(ctx, id)
	if err != nil {
		c.logger.Error("failed to get stats stream", "container_id", id, "error", err)
		return
	}
	defer reader.Close()

	scanner := bufio.NewScanner(reader)
	var parseState ParseState

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := scanner.Bytes()
		pt, newState, err := ParseRawStats(line, &parseState)
		if err != nil {
			c.logger.Error("failed to parse stats JSON", "container_id", id, "error", err)
			continue
		}
		parseState = newState

		c.mu.Lock()
		if buf, ok := c.history[id]; ok {
			buf.Add(pt)
		}
		c.mu.Unlock()

		if c.onStats != nil {
			c.onStats(id, pt)
		}
	}
}

func (c *Collector) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, cancel := range c.cancelFuncs {
		cancel()
		delete(c.cancelFuncs, id)
	}
}
