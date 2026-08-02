package hub

import (
	"log/slog"
	"os"
	"testing"
)

func TestHubMaxConnections(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	h := NewHub(logger, 2)

	c1 := NewClient("client-1", nil, h, 10, nil)
	c2 := NewClient("client-2", nil, h, 10, nil)
	c3 := NewClient("client-3", nil, h, 10, nil)

	if err := h.Register(c1); err != nil {
		t.Fatalf("c1 register failed: %v", err)
	}
	if err := h.Register(c2); err != nil {
		t.Fatalf("c2 register failed: %v", err)
	}

	if err := h.Register(c3); err != ErrMaxConnections {
		t.Errorf("expected ErrMaxConnections, got %v", err)
	}

	if h.ClientCount() != 2 {
		t.Errorf("expected 2 clients, got %d", h.ClientCount())
	}

	h.Unregister(c1)
	if h.ClientCount() != 1 {
		t.Errorf("expected 1 client after unregister, got %d", h.ClientCount())
	}
}

func TestHubSlowClientStatsDrop(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	h := NewHub(logger, 10)

	c := NewClient("slow-client", nil, h, 1, nil)
	_ = h.Register(c)

	h.BroadcastState("stats", 0, map[string]string{"foo": "bar"})
	h.BroadcastState("stats", 0, map[string]string{"foo": "baz"})

	if len(c.Send) != 1 {
		t.Errorf("expected buffer len 1 (stats dropped without error), got %d", len(c.Send))
	}
}
