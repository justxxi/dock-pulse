package supervisor

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/owner/dock-pulse/internal/dockerx"
	"github.com/owner/dock-pulse/internal/protocol"
)

func TestSupervisorContainerExitAndRestart(t *testing.T) {
	t.Parallel()

	fake := dockerx.NewFakeEngine()
	fake.AddContainer(dockerx.Container{
		ID:     "c1",
		Name:   "worker",
		Status: "exited",
		State:  dockerx.State{Running: false, ExitCode: 1},
	})

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	bm := NewBackoffManager(BackoffConfig{
		BaseInterval:       10 * time.Millisecond,
		MaxInterval:        50 * time.Millisecond,
		MaxAttempts:        2,
		StabilityThreshold: 1 * time.Second,
	}, nil, nil)

	eventChan := make(chan protocol.SupervisorEventData, 10)
	sup := NewSupervisor(fake, logger, true, bm, func(evt protocol.SupervisorEventData) {
		eventChan <- evt
	})

	ctx := context.Background()
	c, _ := fake.Inspect(ctx, "c1")

	sup.HandleContainerExit(ctx, c, 1, "test crash")

	select {
	case evt := <-eventChan:
		if evt.Action != "scheduled" || evt.Attempt != 1 {
			t.Errorf("expected scheduled event attempt 1, got action=%s attempt=%d", evt.Action, evt.Attempt)
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("timeout waiting for supervisor scheduled event")
	}

	select {
	case evt := <-eventChan:
		if evt.Action != "restarting" {
			t.Errorf("expected restarting event, got action=%s", evt.Action)
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("timeout waiting for supervisor restarting event")
	}

	time.Sleep(50 * time.Millisecond)

	if calls := fake.GetRestartCalls("c1"); calls != 1 {
		t.Errorf("expected 1 restart call, got %d", calls)
	}
}

func TestSupervisorDisabledLabel(t *testing.T) {
	t.Parallel()

	fake := dockerx.NewFakeEngine()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	bm := NewBackoffManager(DefaultBackoffConfig(), nil, nil)

	eventChan := make(chan protocol.SupervisorEventData, 10)
	sup := NewSupervisor(fake, logger, true, bm, func(evt protocol.SupervisorEventData) {
		eventChan <- evt
	})

	c := dockerx.Container{
		ID:     "c2",
		Labels: map[string]string{"dock-pulse.autorestart": "off"},
	}

	sup.HandleContainerExit(context.Background(), c, 1, "test crash")

	select {
	case evt := <-eventChan:
		t.Errorf("unexpected event received for disabled label: %v", evt)
	default:
	}
}
