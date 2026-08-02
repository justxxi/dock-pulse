package supervisor

import (
	"math/rand/v2"
	"sync"
	"time"
)

type Clock interface {
	Now() time.Time
}

type RealClock struct{}

func (RealClock) Now() time.Time {
	return time.Now()
}

type RandomSource interface {
	Float64() float64
}

type RealRandomSource struct{}

func (RealRandomSource) Float64() float64 {
	return rand.Float64()
}

type BackoffConfig struct {
	BaseInterval       time.Duration
	MaxInterval        time.Duration
	MaxAttempts        int
	StabilityThreshold time.Duration
}

func DefaultBackoffConfig() BackoffConfig {
	return BackoffConfig{
		BaseInterval:       1 * time.Second,
		MaxInterval:        60 * time.Second,
		MaxAttempts:        5,
		StabilityThreshold: 30 * time.Second,
	}
}

type ContainerBackoffState struct {
	Attempts       int
	LastRestartAt  time.Time
	Exhausted      bool
	IntentionalStop bool
}

type BackoffManager struct {
	mu     sync.Mutex
	cfg    BackoffConfig
	clock  Clock
	rand   RandomSource
	states map[string]*ContainerBackoffState
}

func NewBackoffManager(cfg BackoffConfig, clock Clock, randSource RandomSource) *BackoffManager {
	if clock == nil {
		clock = RealClock{}
	}
	if randSource == nil {
		randSource = RealRandomSource{}
	}
	return &BackoffManager{
		cfg:    cfg,
		clock:  clock,
		rand:   randSource,
		states: make(map[string]*ContainerBackoffState),
	}
}

func (bm *BackoffManager) MarkIntentionalStop(containerID string) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	st, ok := bm.states[containerID]
	if !ok {
		st = &ContainerBackoffState{}
		bm.states[containerID] = st
	}
	st.IntentionalStop = true
}

func (bm *BackoffManager) ClearIntentionalStop(containerID string) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if st, ok := bm.states[containerID]; ok {
		st.IntentionalStop = false
	}
}

func (bm *BackoffManager) NextDelay(containerID string) (time.Duration, int, bool) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	st, ok := bm.states[containerID]
	if !ok {
		st = &ContainerBackoffState{}
		bm.states[containerID] = st
	}

	if st.IntentionalStop {
		return 0, st.Attempts, false
	}

	if st.Exhausted {
		return 0, st.Attempts, false
	}

	now := bm.clock.Now()
	if !st.LastRestartAt.IsZero() && now.Sub(st.LastRestartAt) >= bm.cfg.StabilityThreshold {
		st.Attempts = 0
		st.Exhausted = false
	}

	st.Attempts++
	if st.Attempts > bm.cfg.MaxAttempts {
		st.Exhausted = true
		return 0, st.Attempts, false
	}

	st.LastRestartAt = now

	mult := 1 << (st.Attempts - 1)
	rawDelay := time.Duration(mult) * bm.cfg.BaseInterval
	if rawDelay > bm.cfg.MaxInterval {
		rawDelay = bm.cfg.MaxInterval
	}

	jitterFactor := bm.rand.Float64()
	delay := time.Duration(float64(rawDelay) * jitterFactor)

	return delay, st.Attempts, true
}

func (bm *BackoffManager) GetState(containerID string) ContainerBackoffState {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if st, ok := bm.states[containerID]; ok {
		return *st
	}
	return ContainerBackoffState{}
}

func (bm *BackoffManager) Reset(containerID string) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	delete(bm.states, containerID)
}
