package events

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/justxxi/dock-pulse/internal/dockerx"
	"github.com/justxxi/dock-pulse/internal/registry"
)

type Watcher struct {
	engine   dockerx.Engine
	registry *registry.Registry
	logger   *slog.Logger
	onEvent  func(dockerx.Event)
}

func NewWatcher(engine dockerx.Engine, reg *registry.Registry, logger *slog.Logger, onEvent func(dockerx.Event)) *Watcher {
	return &Watcher{
		engine:   engine,
		registry: reg,
		logger:   logger,
		onEvent:  onEvent,
	}
}

func (w *Watcher) Run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	backoff := 500 * time.Millisecond
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		w.logger.Info("starting docker event stream watcher")
		eventChan, errChan := w.engine.Events(ctx)

		connected := true
		for connected {
			select {
			case <-ctx.Done():
				return
			case err, ok := <-errChan:
				if !ok {
					connected = false
					break
				}
				if err != nil {
					w.logger.Error("docker event stream error", "error", err)
				}
				connected = false
			case evt, ok := <-eventChan:
				if !ok {
					connected = false
					break
				}
				backoff = 500 * time.Millisecond

				if err := w.registry.UpdateFromEvent(ctx, evt); err != nil {
					w.logger.Error("failed to process event in registry", "event", evt.Action, "actor_id", evt.ActorID, "error", err)
				}

				if w.onEvent != nil {
					w.onEvent(evt)
				}
			}
		}

		select {
		case <-ctx.Done():
			return
		default:
		}

		jitter := time.Duration(rand.Int64N(int64(backoff / 2)))
		sleepTime := backoff + jitter
		w.logger.Warn("docker event stream disconnected, retrying after backoff", "backoff", sleepTime.String())

		select {
		case <-ctx.Done():
			return
		case <-time.After(sleepTime):
		}

		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}

		if _, err := w.registry.Resync(ctx); err != nil {
			w.logger.Error("failed full resync after event stream reconnection", "error", err)
		} else {
			w.logger.Info("full registry resync completed after event stream reconnection")
		}
	}
}
