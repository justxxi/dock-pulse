package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/owner/dock-pulse/internal/auth"
	"github.com/owner/dock-pulse/internal/dockerx"
	"github.com/owner/dock-pulse/internal/hub"
	"github.com/owner/dock-pulse/internal/logs"
	"github.com/owner/dock-pulse/internal/registry"
	"github.com/owner/dock-pulse/internal/supervisor"
)

func setupTestServer(readOnly bool) (*httptest.Server, *dockerx.FakeEngine) {
	fake := dockerx.NewFakeEngine()
	fake.AddContainer(dockerx.Container{
		ID:     "c100",
		Name:   "test-app",
		Status: "running",
		State:  dockerx.State{Running: true},
	})

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	reg := registry.New(fake, nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = reg.Resync(ctx)

	streamer := logs.NewStreamer(fake, logger, 100, nil)
	bm := supervisor.NewBackoffManager(supervisor.DefaultBackoffConfig(), nil, nil)
	sup := supervisor.NewSupervisor(fake, logger, true, bm, nil)
	h := hub.NewHub(logger, 100)
	authenticator := auth.NewAuthenticator("", false)

	router := NewRouter(fake, reg, streamer, sup, h, authenticator, logger, "/", readOnly)
	return httptest.NewServer(router), fake
}

func TestHealthEndpoint(t *testing.T) {
	t.Parallel()

	ts, _ := setupTestServer(false)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/health")
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestContainersEndpoint(t *testing.T) {
	t.Parallel()

	ts, _ := setupTestServer(false)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/containers")
	if err != nil {
		t.Fatalf("list containers request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var payload struct {
		Containers []dockerx.Container `json:"containers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode payload failed: %v", err)
	}

	if len(payload.Containers) != 1 {
		t.Errorf("expected 1 container, got %d", len(payload.Containers))
	}
}

func TestReadOnlyMode(t *testing.T) {
	t.Parallel()

	ts, _ := setupTestServer(true)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/containers/c100/stop", "application/json", nil)
	if err != nil {
		t.Fatalf("stop request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected status 403 Forbidden in read-only mode, got %d", resp.StatusCode)
	}
}
