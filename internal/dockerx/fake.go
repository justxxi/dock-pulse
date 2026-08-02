package dockerx

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"time"
)

var ErrContainerNotFound = errors.New("container not found")
var ErrEngineUnavailable = errors.New("docker engine unavailable")

type FakeEngine struct {
	mu           sync.Mutex
	containers   map[string]Container
	eventChan    chan Event
	errChan      chan error
	statsReaders map[string]io.ReadCloser
	logsReaders  map[string]io.ReadCloser
	failList     error
	failInspect  error
	failStart    error
	failStop     error
	failRestart  error
	failPing     error
	startCalls   map[string]int
	stopCalls    map[string]int
	restartCalls map[string]int
	closed       bool
}

func NewFakeEngine() *FakeEngine {
	return &FakeEngine{
		containers:   make(map[string]Container),
		eventChan:    make(chan Event, 100),
		errChan:      make(chan error, 1),
		statsReaders: make(map[string]io.ReadCloser),
		logsReaders:  make(map[string]io.ReadCloser),
		startCalls:   make(map[string]int),
		stopCalls:    make(map[string]int),
		restartCalls: make(map[string]int),
	}
}

func (f *FakeEngine) AddContainer(c Container) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.containers[c.ID] = c
}

func (f *FakeEngine) RemoveContainer(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.containers, id)
}

func (f *FakeEngine) EmitEvent(evt Event) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.closed {
		select {
		case f.eventChan <- evt:
		default:
		}
	}
}

func (f *FakeEngine) SetStatsReader(id string, r io.ReadCloser) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statsReaders[id] = r
}

func (f *FakeEngine) SetLogsReader(id string, r io.ReadCloser) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.logsReaders[id] = r
}

func (f *FakeEngine) SetFailList(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failList = err
}

func (f *FakeEngine) SetFailInspect(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failInspect = err
}

func (f *FakeEngine) SetFailStart(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failStart = err
}

func (f *FakeEngine) SetFailStop(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failStop = err
}

func (f *FakeEngine) SetFailRestart(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failRestart = err
}

func (f *FakeEngine) SetFailPing(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failPing = err
}

func (f *FakeEngine) GetStartCalls(id string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.startCalls[id]
}

func (f *FakeEngine) GetStopCalls(id string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.stopCalls[id]
}

func (f *FakeEngine) GetRestartCalls(id string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.restartCalls[id]
}

func (f *FakeEngine) List(ctx context.Context) ([]Container, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failList != nil {
		return nil, f.failList
	}
	result := make([]Container, 0, len(f.containers))
	for _, c := range f.containers {
		result = append(result, c)
	}
	return result, nil
}

func (f *FakeEngine) Inspect(ctx context.Context, id string) (Container, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failInspect != nil {
		return Container{}, f.failInspect
	}
	c, ok := f.containers[id]
	if !ok {
		return Container{}, ErrContainerNotFound
	}
	return c, nil
}

func (f *FakeEngine) Events(ctx context.Context) (<-chan Event, <-chan error) {
	return f.eventChan, f.errChan
}

func (f *FakeEngine) Stats(ctx context.Context, id string) (io.ReadCloser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r, ok := f.statsReaders[id]; ok {
		return r, nil
	}
	return io.NopCloser(bytes.NewReader([]byte("{}"))), nil
}

func (f *FakeEngine) Logs(ctx context.Context, id string, opts LogOptions) (io.ReadCloser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r, ok := f.logsReaders[id]; ok {
		return r, nil
	}
	return io.NopCloser(bytes.NewReader(nil)), nil
}

func (f *FakeEngine) Start(ctx context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failStart != nil {
		return f.failStart
	}
	c, ok := f.containers[id]
	if !ok {
		return ErrContainerNotFound
	}
	c.State.Running = true
	c.State.Status = "running"
	c.Status = "Up 1 second"
	f.containers[id] = c
	f.startCalls[id]++
	return nil
}

func (f *FakeEngine) Stop(ctx context.Context, id string, timeout time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failStop != nil {
		return f.failStop
	}
	c, ok := f.containers[id]
	if !ok {
		return ErrContainerNotFound
	}
	c.State.Running = false
	c.State.Status = "exited"
	c.Status = "Exited (0)"
	f.containers[id] = c
	f.stopCalls[id]++
	return nil
}

func (f *FakeEngine) Restart(ctx context.Context, id string, timeout time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failRestart != nil {
		return f.failRestart
	}
	c, ok := f.containers[id]
	if !ok {
		return ErrContainerNotFound
	}
	c.RestartCount++
	c.State.Running = true
	c.State.Status = "running"
	c.Status = "Up 1 second"
	f.containers[id] = c
	f.restartCalls[id]++
	return nil
}

func (f *FakeEngine) Ping(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failPing != nil {
		return f.failPing
	}
	return nil
}

func (f *FakeEngine) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.closed {
		f.closed = true
		close(f.eventChan)
		close(f.errChan)
	}
	return nil
}
