package httpapi

import (
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/owner/dock-pulse/internal/auth"
	"github.com/owner/dock-pulse/internal/dockerx"
	"github.com/owner/dock-pulse/internal/hub"
	"github.com/owner/dock-pulse/internal/logs"
	"github.com/owner/dock-pulse/internal/registry"
	"github.com/owner/dock-pulse/internal/supervisor"
	"github.com/owner/dock-pulse/internal/web"
)

func NewRouter(
	engine dockerx.Engine,
	reg *registry.Registry,
	streamer *logs.Streamer,
	sup *supervisor.Supervisor,
	h *hub.Hub,
	authenticator *auth.Authenticator,
	logger *slog.Logger,
	basePath string,
	readOnly bool,
) http.Handler {
	mux := http.NewServeMux()
	handlers := NewHandlers(engine, reg, streamer, sup, logger)
	wsHandler := NewWSHandler(h, reg, streamer, authenticator, logger)

	mux.HandleFunc("GET /api/health", handlers.Health)
	mux.HandleFunc("GET /api/version", handlers.Version)
	mux.HandleFunc("GET /api/containers", handlers.ListContainers)
	mux.HandleFunc("GET /api/containers/{id}", handlers.GetContainer)
	mux.HandleFunc("GET /api/containers/{id}/logs", handlers.GetContainerLogs)

	mux.HandleFunc("POST /api/containers/{id}/start", handlers.StartContainer)
	mux.HandleFunc("POST /api/containers/{id}/stop", handlers.StopContainer)
	mux.HandleFunc("POST /api/containers/{id}/restart", handlers.RestartContainer)

	mux.Handle("GET /api/stream", wsHandler)

	webFS, err := web.GetFS()
	if err == nil {
		fileServer := http.FileServer(http.FS(webFS))
		mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				http.NotFound(w, r)
				return
			}

			path := strings.TrimPrefix(r.URL.Path, "/")
			if path == "" {
				path = "index.html"
			}

			f, err := webFS.Open(path)
			if err != nil {
				path = "index.html"
				f, err = webFS.Open(path)
				if err != nil {
					http.NotFound(w, r)
					return
				}
			}

			defer f.Close()

			stat, err := f.Stat()
			if err != nil {
				http.NotFound(w, r)
				return
			}

			if stat.IsDir() {
				path = "index.html"
			}

			if path == "index.html" {
				w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			} else {
				ext := filepath.Ext(path)
				if ext == ".js" || ext == ".css" || ext == ".woff2" {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				} else {
					w.Header().Set("Cache-Control", "public, max-age=3600")
				}
			}

			if seeker, ok := f.(io.ReadSeeker); ok {
				h := sha256.New()
				_, _ = io.Copy(h, seeker)
				seeker.Seek(0, io.SeekStart)
				etag := fmt.Sprintf(`"%x"`, h.Sum(nil)[:8])
				w.Header().Set("ETag", etag)

				if match := r.Header.Get("If-None-Match"); match == etag {
					w.WriteHeader(http.StatusNotModified)
					return
				}
			}

			fileServer.ServeHTTP(w, r)
		})
	}

	chain := MiddlewareChain(logger, authenticator, readOnly)

	if basePath != "" && basePath != "/" {
		rootMux := http.NewServeMux()
		prefix := strings.TrimRight(basePath, "/")
		rootMux.Handle(prefix+"/", http.StripPrefix(prefix, chain(mux)))
		return rootMux
	}

	return chain(mux)
}
