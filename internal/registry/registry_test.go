package registry

import (
	"context"
	"testing"
	"time"

	"github.com/justxxi/dock-pulse/internal/dockerx"
)

func TestRegistryResyncAndFiltering(t *testing.T) {
	t.Parallel()

	fake := dockerx.NewFakeEngine()
	fake.AddContainer(dockerx.Container{ID: "c1", Name: "app-web", Status: "running"})
	fake.AddContainer(dockerx.Container{ID: "c2", Name: "app-db", Status: "running"})
	fake.AddContainer(dockerx.Container{ID: "c3", Name: "redis-cache", Status: "running"})

	reg := New(fake, []string{"app-*"}, []string{"*-db"})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	seq, err := reg.Resync(ctx)
	if err != nil {
		t.Fatalf("Resync failed: %v", err)
	}
	if seq == 0 {
		t.Errorf("expected non-zero sequence")
	}

	list, _ := reg.List()
	if len(list) != 1 {
		t.Fatalf("expected 1 allowed container (app-web), got %d", len(list))
	}
	if list[0].Name != "app-web" {
		t.Errorf("expected container app-web, got %s", list[0].Name)
	}
}

func TestRegistryEventUpdate(t *testing.T) {
	t.Parallel()

	fake := dockerx.NewFakeEngine()
	fake.AddContainer(dockerx.Container{ID: "c1", Name: "app", Status: "running"})

	reg := New(fake, nil, nil)
	ctx := context.Background()

	_, err := reg.Resync(ctx)
	if err != nil {
		t.Fatalf("Resync failed: %v", err)
	}

	fake.RemoveContainer("c1")

	err = reg.UpdateFromEvent(ctx, dockerx.Event{
		Action:  "destroy",
		ActorID: "c1",
	})
	if err != nil {
		t.Fatalf("UpdateFromEvent failed: %v", err)
	}

	list, _ := reg.List()
	if len(list) != 0 {
		t.Errorf("expected 0 containers after destroy event, got %d", len(list))
	}
}
