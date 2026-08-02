package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/justxxi/dock-pulse/internal/dockerx"
	"github.com/justxxi/dock-pulse/internal/logs"
	"github.com/justxxi/dock-pulse/internal/registry"
	"github.com/justxxi/dock-pulse/internal/supervisor"
	"github.com/justxxi/dock-pulse/internal/version"
)

type Handlers struct {
	engine     dockerx.Engine
	registry   *registry.Registry
	streamer   *logs.Streamer
	supervisor *supervisor.Supervisor
	logger     *slog.Logger
}

func NewHandlers(engine dockerx.Engine, reg *registry.Registry, streamer *logs.Streamer, sup *supervisor.Supervisor, logger *slog.Logger) *Handlers {
	return &Handlers{
		engine:     engine,
		registry:   reg,
		streamer:   streamer,
		supervisor: sup,
		logger:     logger,
	}
}

func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	reqID := getReqID(r)
	ctx, cancel := contextWithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	err := h.engine.Ping(ctx)
	status := "healthy"
	dockerStatus := "connected"
	if err != nil {
		status = "unhealthy"
		dockerStatus = "disconnected"
	}

	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     status,
		"docker":     dockerStatus,
		"request_id": reqID,
	})
}

func (h *Handlers) Version(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(version.Get())
}

func (h *Handlers) ListContainers(w http.ResponseWriter, r *http.Request) {
	containers, seq := h.registry.List()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"containers": containers,
		"seq":        seq,
	})
}

func (h *Handlers) GetContainer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	reqID := getReqID(r)

	c, ok := h.registry.Get(id)
	if !ok {
		writeJSONError(w, http.StatusNotFound, "container_not_found", "Container with specified ID was not found", reqID)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(c)
}

func (h *Handlers) GetContainerLogs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	reqID := getReqID(r)

	if _, ok := h.registry.Get(id); !ok {
		writeJSONError(w, http.StatusNotFound, "container_not_found", "Container with specified ID was not found", reqID)
		return
	}

	tailStr := r.URL.Query().Get("tail")
	tail := 200
	if tailStr != "" {
		if n, err := strconv.Atoi(tailStr); err == nil && n > 0 {
			tail = n
		}
	}

	ring := h.streamer.GetRingBuffer(id)
	lines := ring.ReadTail(tail)
	if lines == nil {
		lines = []logs.LogLine{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"container_id": id,
		"lines":        lines,
	})
}

func (h *Handlers) StartContainer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	reqID := getReqID(r)

	c, ok := h.registry.Get(id)
	if !ok {
		writeJSONError(w, http.StatusNotFound, "container_not_found", "Container with specified ID was not found", reqID)
		return
	}

	h.supervisor.ClearIntentionalStop(c.ID)

	ctx, cancel := contextWithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := h.engine.Start(ctx, c.ID); err != nil {
		h.logger.Error("failed to start container", "container_id", c.ID, "request_id", reqID, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "start_failed", "Failed to start container", reqID)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "started", "id": c.ID})
}

func (h *Handlers) StopContainer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	reqID := getReqID(r)

	c, ok := h.registry.Get(id)
	if !ok {
		writeJSONError(w, http.StatusNotFound, "container_not_found", "Container with specified ID was not found", reqID)
		return
	}

	h.supervisor.MarkIntentionalStop(c.ID)

	ctx, cancel := contextWithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	if err := h.engine.Stop(ctx, c.ID, 10*time.Second); err != nil {
		h.logger.Error("failed to stop container", "container_id", c.ID, "request_id", reqID, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "stop_failed", "Failed to stop container", reqID)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "stopped", "id": c.ID})
}

func (h *Handlers) RestartContainer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	reqID := getReqID(r)

	c, ok := h.registry.Get(id)
	if !ok {
		writeJSONError(w, http.StatusNotFound, "container_not_found", "Container with specified ID was not found", reqID)
		return
	}

	h.supervisor.ClearIntentionalStop(c.ID)

	ctx, cancel := contextWithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	if err := h.engine.Restart(ctx, c.ID, 10*time.Second); err != nil {
		h.logger.Error("failed to restart container", "container_id", c.ID, "request_id", reqID, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "restart_failed", "Failed to restart container", reqID)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "restarted", "id": c.ID})
}

func getReqID(r *http.Request) string {
	if id, ok := r.Context().Value(RequestIDKey).(string); ok {
		return id
	}
	return ""
}

func contextWithTimeout(parent context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, d)
}
