package supervisor

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/justxxi/dock-pulse/internal/dockerx"
	"github.com/justxxi/dock-pulse/internal/protocol"
)

type Supervisor struct {
	mu             sync.Mutex
	engine         dockerx.Engine
	logger         *slog.Logger
	enabled        bool
	backoff        *BackoffManager
	onEvent        func(evt protocol.SupervisorEventData)
	pendingRestarts map[string]context.CancelFunc
}

func NewSupervisor(engine dockerx.Engine, logger *slog.Logger, enabled bool, backoff *BackoffManager, onEvent func(evt protocol.SupervisorEventData)) *Supervisor {
	return &Supervisor{
		engine:          engine,
		logger:          logger,
		enabled:         enabled,
		backoff:         backoff,
		onEvent:         onEvent,
		pendingRestarts: make(map[string]context.CancelFunc),
	}
}

func (s *Supervisor) MarkIntentionalStop(id string) {
	s.backoff.MarkIntentionalStop(id)
	s.cancelPending(id)
}

func (s *Supervisor) ClearIntentionalStop(id string) {
	s.backoff.ClearIntentionalStop(id)
}

func (s *Supervisor) HandleContainerExit(ctx context.Context, c dockerx.Container, exitCode int, reason string) {
	if !s.enabled {
		return
	}

	if val, ok := c.Labels["dock-pulse.autorestart"]; ok {
		if strings.ToLower(val) == "off" || strings.ToLower(val) == "false" || val == "0" {
			return
		}
	}

	for _, nativePolicy := range []string{"always", "unless-stopped"} {
		if strings.Contains(c.Status, nativePolicy) {
			return
		}
	}

	st := s.backoff.GetState(c.ID)
	if st.IntentionalStop {
		s.logger.Info("ignoring exit for intentionally stopped container", "container_id", c.ID)
		return
	}

	delay, attempt, ok := s.backoff.NextDelay(c.ID)
	if !ok {
		s.logger.Warn("supervisor restart attempts exhausted", "container_id", c.ID, "attempts", attempt)
		if s.onEvent != nil {
			s.onEvent(protocol.SupervisorEventData{
				ContainerID: c.ID,
				Action:      "exhausted",
				Attempt:     attempt,
				Reason:      reason,
				Exhausted:   true,
			})
		}
		return
	}

	nextRetry := time.Now().Add(delay)
	s.logger.Info("scheduling supervisor auto-restart", "container_id", c.ID, "attempt", attempt, "delay", delay.String())

	if s.onEvent != nil {
		s.onEvent(protocol.SupervisorEventData{
			ContainerID: c.ID,
			Action:      "scheduled",
			Attempt:     attempt,
			NextRetry:   nextRetry,
			Reason:      reason,
			Exhausted:   false,
		})
	}

	s.cancelPending(c.ID)

	rCtx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	s.pendingRestarts[c.ID] = cancel
	s.mu.Unlock()

	go func() {
		defer func() {
			s.mu.Lock()
			delete(s.pendingRestarts, c.ID)
			s.mu.Unlock()
		}()

		select {
		case <-rCtx.Done():
			return
		case <-time.After(delay):
		}

		s.logger.Info("executing supervisor restart", "container_id", c.ID, "attempt", attempt)
		if s.onEvent != nil {
			s.onEvent(protocol.SupervisorEventData{
				ContainerID: c.ID,
				Action:      "restarting",
				Attempt:     attempt,
				Reason:      reason,
				Exhausted:   false,
			})
		}

		err := s.engine.Restart(rCtx, c.ID, 10*time.Second)
		if err != nil {
			s.logger.Error("supervisor restart failed", "container_id", c.ID, "attempt", attempt, "error", err)
			s.HandleContainerExit(ctx, c, -1, fmt.Sprintf("restart failed: %v", err))
		} else {
			s.logger.Info("supervisor restart succeeded", "container_id", c.ID, "attempt", attempt)
		}
	}()
}

func (s *Supervisor) cancelPending(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cancel, ok := s.pendingRestarts[id]; ok {
		cancel()
		delete(s.pendingRestarts, id)
	}
}

func ParseMaxAttempts(c dockerx.Container, defaultMax int) int {
	if val, ok := c.Labels["dock-pulse.autorestart.max"]; ok {
		if n, err := strconv.Atoi(val); err == nil && n > 0 {
			return n
		}
	}
	return defaultMax
}
