package dockerx

import (
	"context"
	"io"
	"time"
)

type Port struct {
	IP          string `json:"ip,omitempty"`
	PrivatePort uint16 `json:"private_port"`
	PublicPort  uint16 `json:"public_port,omitempty"`
	Type        string `json:"type"`
}

type State struct {
	Status     string    `json:"status"`
	Running    bool      `json:"running"`
	Paused     bool      `json:"paused"`
	Restarting bool      `json:"restarting"`
	OOMKilled  bool      `json:"oom_killed"`
	Dead       bool      `json:"dead"`
	Pid        int       `json:"pid"`
	ExitCode   int       `json:"exit_code"`
	Error      string    `json:"error"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
}

type Container struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Image        string            `json:"image"`
	ImageID      string            `json:"image_id"`
	Command      string            `json:"command"`
	Created      time.Time         `json:"created"`
	State        State             `json:"state"`
	Status       string            `json:"status"`
	Ports        []Port            `json:"ports"`
	Labels       map[string]string `json:"labels"`
	Mounts       []string          `json:"mounts"`
	Network      string            `json:"network"`
	RestartCount int               `json:"restart_count"`
}

type Event struct {
	Type       string            `json:"type"`
	Action     string            `json:"action"`
	ActorID    string            `json:"actor_id"`
	ActorName  string            `json:"actor_name"`
	Attributes map[string]string `json:"attributes"`
	Time       time.Time         `json:"time"`
}

type LogOptions struct {
	ShowStdout bool
	ShowStderr bool
	Since      time.Time
	Until      time.Time
	Timestamps bool
	Follow     bool
	Tail       string
}

type Engine interface {
	List(ctx context.Context) ([]Container, error)
	Inspect(ctx context.Context, id string) (Container, error)
	Events(ctx context.Context) (<-chan Event, <-chan error)
	Stats(ctx context.Context, id string) (io.ReadCloser, error)
	Logs(ctx context.Context, id string, opts LogOptions) (io.ReadCloser, error)
	Start(ctx context.Context, id string) error
	Stop(ctx context.Context, id string, timeout time.Duration) error
	Restart(ctx context.Context, id string, timeout time.Duration) error
	Ping(ctx context.Context) error
	Close() error
}
