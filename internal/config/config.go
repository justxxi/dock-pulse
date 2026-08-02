package config

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

var ErrInvalidConfig = errors.New("invalid configuration")

type Config struct {
	ListenAddr        string        `json:"listen_addr"`
	DockerHost        string        `json:"docker_host"`
	Token             string        `json:"token"`
	TLSCert           string        `json:"tls_cert"`
	TLSKey            string        `json:"tls_key"`
	StatsInterval     time.Duration `json:"stats_interval"`
	LogRingSize       int           `json:"log_ring_size"`
	SupervisorEnabled bool          `json:"supervisor_enabled"`
	MaxWSConnections  int           `json:"max_ws_connections"`
	LogLevel          string        `json:"log_level"`
	BasePath          string        `json:"base_path"`
	ReadOnly          bool          `json:"read_only"`
	AllowContainers   []string      `json:"allow_containers"`
	DenyContainers    []string      `json:"deny_containers"`
}

func DefaultConfig() Config {
	return Config{
		ListenAddr:        ":8080",
		DockerHost:        "",
		Token:             "",
		TLSCert:           "",
		TLSKey:            "",
		StatsInterval:     2 * time.Second,
		LogRingSize:       1000,
		SupervisorEnabled: true,
		MaxWSConnections:  100,
		LogLevel:          "info",
		BasePath:          "/",
		ReadOnly:          false,
		AllowContainers:   nil,
		DenyContainers:    nil,
	}
}

func Load(args []string) (Config, error) {
	cfg := DefaultConfig()

	fs := flag.NewFlagSet("dock-pulse", flag.ContinueOnError)

	var allowCSV, denyCSV string

	fs.StringVar(&cfg.ListenAddr, "listen-addr", cfg.ListenAddr, "Address to listen on")
	fs.StringVar(&cfg.DockerHost, "docker-host", cfg.DockerHost, "Docker daemon socket or URL")
	fs.StringVar(&cfg.Token, "token", cfg.Token, "Bearer authentication token")
	fs.StringVar(&cfg.TLSCert, "tls-cert", cfg.TLSCert, "Path to TLS certificate file")
	fs.StringVar(&cfg.TLSKey, "tls-key", cfg.TLSKey, "Path to TLS private key file")
	fs.DurationVar(&cfg.StatsInterval, "stats-interval", cfg.StatsInterval, "Interval for collecting container metrics")
	fs.IntVar(&cfg.LogRingSize, "log-ring-size", cfg.LogRingSize, "Capacity of in-memory log ring buffer per container")
	fs.BoolVar(&cfg.SupervisorEnabled, "supervisor", cfg.SupervisorEnabled, "Enable supervisor auto-restart")
	fs.IntVar(&cfg.MaxWSConnections, "max-ws-connections", cfg.MaxWSConnections, "Maximum concurrent WebSocket clients")
	fs.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "Logging level (debug, info, warn, error)")
	fs.StringVar(&cfg.BasePath, "base-path", cfg.BasePath, "Base URL path prefix")
	fs.BoolVar(&cfg.ReadOnly, "read-only", cfg.ReadOnly, "Disable mutative container actions")
	fs.StringVar(&allowCSV, "allow-containers", "", "Comma-separated allowed container names or patterns")
	fs.StringVar(&denyCSV, "deny-containers", "", "Comma-separated denied container names or patterns")

	if env := os.Getenv("DOCK_PULSE_LISTEN_ADDR"); env != "" {
		cfg.ListenAddr = env
	}
	if env := os.Getenv("DOCK_PULSE_DOCKER_HOST"); env != "" {
		cfg.DockerHost = env
	}
	if env := os.Getenv("DOCK_PULSE_TOKEN"); env != "" {
		cfg.Token = env
	}
	if env := os.Getenv("DOCK_PULSE_TLS_CERT"); env != "" {
		cfg.TLSCert = env
	}
	if env := os.Getenv("DOCK_PULSE_TLS_KEY"); env != "" {
		cfg.TLSKey = env
	}
	if env := os.Getenv("DOCK_PULSE_STATS_INTERVAL"); env != "" {
		if d, err := time.ParseDuration(env); err == nil {
			cfg.StatsInterval = d
		}
	}
	if env := os.Getenv("DOCK_PULSE_LOG_RING_SIZE"); env != "" {
		if n, err := strconv.Atoi(env); err == nil {
			cfg.LogRingSize = n
		}
	}
	if env := os.Getenv("DOCK_PULSE_SUPERVISOR"); env != "" {
		if b, err := strconv.ParseBool(env); err == nil {
			cfg.SupervisorEnabled = b
		}
	}
	if env := os.Getenv("DOCK_PULSE_MAX_WS_CONNECTIONS"); env != "" {
		if n, err := strconv.Atoi(env); err == nil {
			cfg.MaxWSConnections = n
		}
	}
	if env := os.Getenv("DOCK_PULSE_LOG_LEVEL"); env != "" {
		cfg.LogLevel = env
	}
	if env := os.Getenv("DOCK_PULSE_BASE_PATH"); env != "" {
		cfg.BasePath = env
	}
	if env := os.Getenv("DOCK_PULSE_READ_ONLY"); env != "" {
		if b, err := strconv.ParseBool(env); err == nil {
			cfg.ReadOnly = b
		}
	}
	if env := os.Getenv("DOCK_PULSE_ALLOW_CONTAINERS"); env != "" {
		allowCSV = env
	}
	if env := os.Getenv("DOCK_PULSE_DENY_CONTAINERS"); env != "" {
		denyCSV = env
	}

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	if allowCSV != "" {
		cfg.AllowContainers = splitCSV(allowCSV)
	}
	if denyCSV != "" {
		cfg.DenyContainers = splitCSV(denyCSV)
	}

	cfg.BasePath = normalizeBasePath(cfg.BasePath)

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	var errs []string

	if c.ListenAddr == "" {
		errs = append(errs, "listen-addr cannot be empty")
	}

	if isNonLoopback(c.ListenAddr) && c.Token == "" {
		errs = append(errs, "token is required when listening on non-loopback address")
	}

	if (c.TLSCert != "" && c.TLSKey == "") || (c.TLSCert == "" && c.TLSKey != "") {
		errs = append(errs, "both tls-cert and tls-key must be provided for TLS")
	}

	if c.StatsInterval <= 0 {
		errs = append(errs, "stats-interval must be greater than zero")
	}

	if c.LogRingSize <= 0 {
		errs = append(errs, "log-ring-size must be greater than zero")
	}

	if c.MaxWSConnections <= 0 {
		errs = append(errs, "max-ws-connections must be greater than zero")
	}

	switch strings.ToLower(c.LogLevel) {
	case "debug", "info", "warn", "error":
	default:
		errs = append(errs, fmt.Sprintf("invalid log-level: %s", c.LogLevel))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func isNonLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return true
	}
	return !ip.IsLoopback()
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	res := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			res = append(res, trimmed)
		}
	}
	return res
}

func normalizeBasePath(p string) string {
	if p == "" || p == "/" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return strings.TrimRight(p, "/")
}
