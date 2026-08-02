package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/owner/dock-pulse/internal/auth"
	"github.com/owner/dock-pulse/internal/config"
	"github.com/owner/dock-pulse/internal/dockerx"
	"github.com/owner/dock-pulse/internal/events"
	"github.com/owner/dock-pulse/internal/httpapi"
	"github.com/owner/dock-pulse/internal/hub"
	"github.com/owner/dock-pulse/internal/logs"
	"github.com/owner/dock-pulse/internal/protocol"
	"github.com/owner/dock-pulse/internal/registry"
	"github.com/owner/dock-pulse/internal/stats"
	"github.com/owner/dock-pulse/internal/supervisor"
	"github.com/owner/dock-pulse/internal/version"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "dock-pulse: %v\n", err)
		var exitCode int
		if errors.Is(err, config.ErrInvalidConfig) {
			exitCode = 2
		} else {
			exitCode = 1
		}
		os.Exit(exitCode)
	}
}

func run(args []string) error {
	cfg, err := config.Load(args)
	if err != nil {
		return fmt.Errorf("%w: %v", config.ErrInvalidConfig, err)
	}

	var level slog.Level
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	logger.Info("starting dock-pulse", "version", version.Version, "listen_addr", cfg.ListenAddr)

	dockerEngine, err := dockerx.NewClient(cfg.DockerHost)
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}
	defer dockerEngine.Close()

	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pingCancel()

	if err := dockerEngine.Ping(pingCtx); err != nil {
		return fmt.Errorf("failed to ping docker daemon: %w", err)
	}
	logger.Info("connected to docker daemon successfully")

	h := hub.NewHub(logger, cfg.MaxWSConnections)
	reg := registry.New(dockerEngine, cfg.AllowContainers, cfg.DenyContainers)

	reg.SetCallbacks(
		func(c dockerx.Container) {
			h.BroadcastState(protocol.TypeContainerUpdated, reg.Seq(), protocol.ContainerUpdatedData{Container: c})
		},
		func(id string) {
			h.BroadcastState(protocol.TypeContainerRemoved, reg.Seq(), protocol.ContainerRemovedData{ID: id})
		},
	)

	collector := stats.NewCollector(
		dockerEngine,
		logger,
		cfg.StatsInterval,
		60,
		20,
		func(id string, pt protocol.StatsPoint) {
			h.BroadcastState(protocol.TypeStats, 0, protocol.ContainerStatsData{ID: id, Stats: pt})
		},
	)
	defer collector.Close()

	streamer := logs.NewStreamer(
		dockerEngine,
		logger,
		cfg.LogRingSize,
		func(containerID string, line logs.LogLine) {
			h.BroadcastLog(containerID, protocol.LogLineData{
				ContainerID: containerID,
				Seq:         line.Seq,
				Timestamp:   line.Timestamp,
				Stream:      line.Stream,
				Text:        line.Text,
			})
		},
	)

	backoffMgr := supervisor.NewBackoffManager(supervisor.DefaultBackoffConfig(), nil, nil)
	sup := supervisor.NewSupervisor(
		dockerEngine,
		logger,
		cfg.SupervisorEnabled,
		backoffMgr,
		func(evt protocol.SupervisorEventData) {
			h.BroadcastState(protocol.TypeSupervisor, 0, evt)
		},
	)

	initialCtx, initialCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer initialCancel()

	if _, err := reg.Resync(initialCtx); err != nil {
		logger.Error("initial container registry sync failed", "error", err)
	}

	containers, _ := reg.List()
	for _, c := range containers {
		if c.State.Running {
			collector.StartCollecting(context.Background(), c.ID)
		}
	}

	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()

	var wg sync.WaitGroup

	watcher := events.NewWatcher(dockerEngine, reg, logger, func(evt dockerx.Event) {
		switch evt.Action {
		case "start":
			collector.StartCollecting(appCtx, evt.ActorID)
		case "die", "stop", "destroy":
			collector.StopCollecting(evt.ActorID)
			if evt.Action == "die" || evt.Action == "exited" {
				c, ok := reg.Get(evt.ActorID)
				if ok {
					exitCode := 0
					if codeStr, hasCode := evt.Attributes["exitCode"]; hasCode {
						fmt.Sscanf(codeStr, "%d", &exitCode)
					}
					if exitCode != 0 {
						sup.HandleContainerExit(appCtx, c, exitCode, "container died with non-zero exit code")
					}
				}
			}
		}
	})

	wg.Add(1)
	go watcher.Run(appCtx, &wg)

	isTLS := cfg.TLSCert != "" && cfg.TLSKey != ""
	authenticator := auth.NewAuthenticator(cfg.Token, isTLS)

	router := httpapi.NewRouter(
		dockerEngine,
		reg,
		streamer,
		sup,
		h,
		authenticator,
		logger,
		cfg.BasePath,
		cfg.ReadOnly,
	)

	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("http server starting", "addr", cfg.ListenAddr, "tls", isTLS)
		var err error
		if isTLS {
			err = server.ListenAndServeTLS(cfg.TLSCert, cfg.TLSKey)
		} else {
			err = server.ListenAndServe()
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		appCancel()
		wg.Wait()
		return fmt.Errorf("server error: %w", err)
	case sig := <-sigChan:
		logger.Info("shutdown signal received", "signal", sig.String())
	}

	appCancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("http server graceful shutdown failed", "error", err)
	}

	wg.Wait()
	logger.Info("dock-pulse stopped cleanly")
	return nil
}
