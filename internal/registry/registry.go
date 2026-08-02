package registry

import (
	"context"
	"fmt"
	"path"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/owner/dock-pulse/internal/dockerx"
)

type Registry struct {
	mu        sync.RWMutex
	engine    dockerx.Engine
	store     map[string]dockerx.Container
	seq       atomic.Uint64
	allows    []string
	denies    []string
	onUpdate  func(c dockerx.Container)
	onRemove  func(id string)
}

func New(engine dockerx.Engine, allows, denies []string) *Registry {
	return &Registry{
		engine: engine,
		store:  make(map[string]dockerx.Container),
		allows: allows,
		denies: denies,
	}
}

func (r *Registry) SetCallbacks(onUpdate func(c dockerx.Container), onRemove func(id string)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onUpdate = onUpdate
	r.onRemove = onRemove
}

func (r *Registry) IsAllowed(c dockerx.Container) bool {
	if len(r.allows) > 0 {
		matched := false
		for _, pattern := range r.allows {
			if matchContainerPattern(c, pattern) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	for _, pattern := range r.denies {
		if matchContainerPattern(c, pattern) {
			return false
		}
	}

	return true
}

func matchContainerPattern(c dockerx.Container, pattern string) bool {
	if pattern == "" {
		return false
	}
	if matched, _ := path.Match(pattern, c.Name); matched {
		return true
	}
	if matched, _ := path.Match(pattern, c.ID); matched {
		return true
	}
	if strings.Contains(c.Name, pattern) || strings.HasPrefix(c.ID, pattern) {
		return true
	}
	for k, v := range c.Labels {
		if k == pattern || fmt.Sprintf("%s=%s", k, v) == pattern {
			return true
		}
	}
	return false
}

func (r *Registry) Resync(ctx context.Context) (uint64, error) {
	containers, err := r.engine.List(ctx)
	if err != nil {
		return r.seq.Load(), fmt.Errorf("list docker containers: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	newMap := make(map[string]dockerx.Container, len(containers))
	for _, c := range containers {
		if r.IsAllowed(c) {
			newMap[c.ID] = c
		}
	}

	for oldID := range r.store {
		if _, exists := newMap[oldID]; !exists {
			if r.onRemove != nil {
				r.onRemove(oldID)
			}
		}
	}

	for id, c := range newMap {
		r.store[id] = c
		if r.onUpdate != nil {
			r.onUpdate(c)
		}
	}

	r.store = newMap
	seq := r.seq.Add(1)
	return seq, nil
}

func (r *Registry) UpdateFromEvent(ctx context.Context, evt dockerx.Event) error {
	id := evt.ActorID
	if id == "" {
		return nil
	}

	switch evt.Action {
	case "destroy", "die", "kill", "stop":
		if evt.Action == "destroy" {
			r.mu.Lock()
			delete(r.store, id)
			r.seq.Add(1)
			cb := r.onRemove
			r.mu.Unlock()

			if cb != nil {
				cb(id)
			}
			return nil
		}
	}

	c, err := r.engine.Inspect(ctx, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "No such container") {
			r.mu.Lock()
			delete(r.store, id)
			r.seq.Add(1)
			cb := r.onRemove
			r.mu.Unlock()

			if cb != nil {
				cb(id)
			}
			return nil
		}
		return fmt.Errorf("inspect container for event %s: %w", id, err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.IsAllowed(c) {
		if _, exists := r.store[id]; exists {
			delete(r.store, id)
			r.seq.Add(1)
			if r.onRemove != nil {
				r.onRemove(id)
			}
		}
		return nil
	}

	r.store[id] = c
	r.seq.Add(1)
	if r.onUpdate != nil {
		r.onUpdate(c)
	}

	return nil
}

func (r *Registry) List() ([]dockerx.Container, uint64) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]dockerx.Container, 0, len(r.store))
	for _, c := range r.store {
		result = append(result, c)
	}
	return result, r.seq.Load()
}

func (r *Registry) Get(id string) (dockerx.Container, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	c, ok := r.store[id]
	return c, ok
}

func (r *Registry) Seq() uint64 {
	return r.seq.Load()
}
